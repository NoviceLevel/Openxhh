package xhh

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"openxhh/db"
	"openxhh/loger"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
)

var lock = &sync.Mutex{}

func Reply(text, link_id, reply_id, root_id, iscy string) (isok bool) {
	return createComment(text, link_id, reply_id, root_id, iscy, "")
}

func ReplyImage(text, linkID, replyID, rootID, imageURL string) bool {
	return createComment(text, linkID, replyID, rootID, "0", imageURL)
}

func CommentPostImage(text, linkID, imageURL string) bool {
	return createComment(text, linkID, "-1", "-1", "0", imageURL)
}

func CommentCreateFormData(text, linkID, replyID, rootID, iscy, imageURL string) url.Values {
	if iscy == "" {
		iscy = "0"
	}
	form := url.Values{}
	form.Set("is_cy", iscy)
	form.Set("link_id", linkID)
	form.Set("reply_id", replyID)
	form.Set("root_id", rootID)
	form.Set("text", text)
	form.Set("imgs", imageURL)
	return form
}

func createComment(text, link_id, reply_id, root_id, iscy, imageURL string) (isok bool) {
	lock.Lock()
	defer lock.Unlock()
	Path := "/bbs/app/comment/create"
	Body := CommentCreateFormData(text, link_id, reply_id, root_id, iscy, imageURL).Encode()
	resp := SendReq("POST", Path, bytes.NewReader([]byte(Body)), "")
	if resp == nil {
		loger.Loger.Error("[XHH]链接发送失败了")
		return
	}
	var resps struct {
		Status string `json:"status"`
		Msg    string `json:"msg"`
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		loger.Loger.Error("[XHH]无法解析Body", zap.Error(err))
		return false
	}
	err = json.Unmarshal(data, &resps)
	if err != nil {
		loger.Loger.Error("[XHH]无法反序列化", zap.String("body", string(data)), zap.Error(err))
		return false
	}
	if resps.Status != "ok" {
		if resps.Status == "failed" {
			CommentID, err := strconv.Atoi(reply_id)
			if err != nil {
				loger.Loger.Fatal("[XHH]不可能发生的事", zap.Error(err), zap.Any("info", resps), zap.Any("errs", reply_id))
			}
			db.Replyed(CommentID)
			loger.Loger.Warn("[XHH]异常发送：AI回复已生成但评论发送失败，已标记完成避免重复发送", zap.Any("Resp", resps), zap.String("link_id", link_id), zap.String("reply_id", reply_id), zap.String("root_id", root_id))
			time.Sleep(5 * time.Second)
			return true
		}
		if resps.Msg == "评论已被删除" {
			time.Sleep(5 * time.Second)
			return true
		}
		loger.Loger.Error("[XHH]评论发送失败", zap.Any("info", resps))
		return false
	}
	time.Sleep(5 * time.Second)
	return true
}
