package xhh

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"
	"xhhrobot/ai"
	"xhhrobot/config"
	"xhhrobot/db"
	"xhhrobot/loger"
)

var Info struct {
	Cookie   string `json:"cookie"`
	HeyBoxId string `json:"heyboxId"`
	Time     int    `json:"time"`
}
var CheckTime int
var ReplyTime int

func Init() {
	file, err := os.ReadFile("./cookie.json")
	if err != nil {
		loger.Loger.Info("[XHH]未检测到Cookie")
		return
	}
	CheckTime = config.ConfigStruct.Xhh.CheckTime
	ReplyTime = config.ConfigStruct.Xhh.ReplyTime
	if CheckTime == 0 {
		loger.Loger.Warn("[XHH]您的设置中未设置检查时间，已默认为30s")
		CheckTime = 30
	}
	if ReplyTime == 0 {
		loger.Loger.Warn("[XHH]您的设置中未设置回复间隔，已默认为10s")
		ReplyTime = 10
	}
	json.Unmarshal(file, &Info)
}

type Msg struct {
	CommentID     int    `json:"comment_a_id"`
	CommentText   string `json:"comment_a_text"`
	MsgID         int    `json:"message_id"`
	RootCommentID int    `json:"root_comment_id"`
	LinkID        int    `json:"linkid"`
	UserID        int    `json:"userid_a"`
}
type Respo struct {
	Msg    string `json:"msg"`
	Result struct {
		Messages []Msg `json:"messages"`
	} `json:"result"`
	Stat    string `json:"stat"`
	Version string `json:"version"`
}

var DontReply bool

func CheckAt() {
	fmt.Println("[XHH]检查@", time.Now().Unix())
	var offset int
	nomore := "false"
	other := fmt.Sprintf("?message_type=16&offset=%v&limit=20&no_more=%s", offset, nomore)
	resp := SendReq("GET", "/bbs/app/user/message", nil, other)
	var data Respo
	Dbyte, err := io.ReadAll(resp.Body)
	if err != nil {
		loger.Loger.Error("[XHH]无法读取Body")
		return
	}
	err = json.Unmarshal(Dbyte, &data)

	if err != nil {
		loger.Loger.Error("[XHH]无法反序列化")
		return
	}

	for _, v := range data.Result.Messages {
		if Check(v.UserID) {
			if DontReply {
				db.Insert(v.MsgID, v.CommentID, v.RootCommentID, v.LinkID, v.UserID, v.CommentText, true)
			} else {
				db.Insert(v.MsgID, v.CommentID, v.RootCommentID, v.LinkID, v.UserID, v.CommentText, false)
			}
		}
	}
	DontReply = false
	time.Sleep(time.Duration(CheckTime) * time.Second)
	CheckAt()
}

func AutoReply() {
	Arr := db.GetComm()
	if len(Arr) == 0 {
		fmt.Println("[XHH]无可回复", time.Now().Unix())
		time.Sleep(time.Duration(ReplyTime) * time.Second)
		AutoReply()
	}
	var wg sync.WaitGroup

	wg.Add(len(Arr))
	for _, v := range Arr {
		go func() {
			defer wg.Done()
			if v.CommentID != 0 {
				var isok bool
				if Check(v.Uid) {
					Info := GetLinkInfo(v.LinkID)
					if Info == "" {
						loger.Loger.Info("[XHH]获取LinkID失败")
						return
					}
					ReplyText := ai.Grok(Info, v.Text)
					if ReplyText == "" {
						loger.Loger.Info("[XHH]Ai返回错误")
						return
					}
					isok = Reply(ReplyText, strconv.Itoa(v.LinkID), strconv.Itoa(v.CommentID), strconv.Itoa(v.RootID), "")

				}
				if isok {
					db.Replyed(v.CommentID)
				} else {
					loger.Loger.Error("[XHH]无法回复评论")
				}
			} else {
				wg.Done()
				fmt.Println("[XHH]无事可做")
			}
		}()
	}
	wg.Wait()
	AutoReply()
}
