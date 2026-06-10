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
	return createComment("ai_reply", text, link_id, reply_id, root_id, iscy, "")
}

func ReplyImage(text, linkID, replyID, rootID, imageURL string) bool {
	return createComment("image_reply", text, linkID, replyID, rootID, "0", imageURL)
}

func CommentPostImage(text, linkID, imageURL string) bool {
	return createComment("image_post", text, linkID, "-1", "-1", "0", imageURL)
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

type commentCreateResult struct {
	Sent    bool
	Handled bool
}

func createComment(source, text, link_id, reply_id, root_id, iscy, imageURL string) (isok bool) {
	return createCommentResult(source, text, link_id, reply_id, root_id, iscy, imageURL).Handled
}

func createCommentSent(source, text, link_id, reply_id, root_id, iscy, imageURL string) bool {
	return createCommentResult(source, text, link_id, reply_id, root_id, iscy, imageURL).Sent
}

func createCommentResult(source, text, link_id, reply_id, root_id, iscy, imageURL string) commentCreateResult {
	if xhhCaptchaCoolingDown("comment_create", zap.String("source", source), zap.String("link_id", link_id), zap.String("reply_id", reply_id), zap.String("root_id", root_id)) {
		return commentCreateResult{}
	}
	lock.Lock()
	defer lock.Unlock()
	Path := "/bbs/app/comment/create"
	Body := CommentCreateFormData(text, link_id, reply_id, root_id, iscy, imageURL).Encode()
	resp := SendReq("POST", Path, bytes.NewReader([]byte(Body)), "")
	if resp == nil {
		loger.Loger.Error("[XHH]链接发送失败了")
		return commentCreateResult{}
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		loger.Loger.Error("[XHH]无法解析Body", zap.Error(err), zap.Int("status", resp.StatusCode), zap.String("link_id", link_id), zap.String("reply_id", reply_id), zap.String("root_id", root_id), zap.Bool("has_image", imageURL != ""))
		return commentCreateResult{}
	}
	if !isHTTPSuccess(resp.StatusCode) {
		body := string(data)
		loger.Loger.Error("[XHH]评论发送 HTTP 失败", zap.Int("status", resp.StatusCode), zap.String("link_id", link_id), zap.String("reply_id", reply_id), zap.String("root_id", root_id), zap.Bool("has_image", imageURL != ""), zap.String("body", readableXHHResponseBody(body)))
		handleXHHHTTPFailure("comment_create", resp.StatusCode, body, zap.String("link_id", link_id), zap.String("reply_id", reply_id), zap.String("root_id", root_id), zap.Bool("has_image", imageURL != ""))
		return commentCreateResult{}
	}
	status, msg, commentID, createdAt := parseCommentCreateResponse(data)
	if status == "" {
		loger.Loger.Error("[XHH]无法反序列化", zap.String("body", readableXHHResponseBody(string(data))))
		return commentCreateResult{}
	}
	if status != "ok" {
		if status == "failed" {
			CommentID, err := strconv.Atoi(reply_id)
			if err != nil || CommentID <= 0 {
				loger.Loger.Error("[XHH]评论发送失败且 reply_id 无法解析", zap.String("info", readableXHHResponseBody(string(data))), zap.String("reply_id", reply_id))
				return commentCreateResult{Sent: false, Handled: false}
			}
			db.Replyed(CommentID)
			loger.Loger.Warn("[XHH]异常发送：AI回复已生成但评论发送失败，已标记完成避免重复发送", zap.String("Resp", readableXHHResponseBody(string(data))), zap.String("msg", msg), zap.String("link_id", link_id), zap.String("reply_id", reply_id), zap.String("root_id", root_id))
			time.Sleep(5 * time.Second)
			return commentCreateResult{Sent: false, Handled: true}
		}
		if msg == "评论已被删除" {
			time.Sleep(5 * time.Second)
			return commentCreateResult{Sent: false, Handled: true}
		}
		loger.Loger.Error("[XHH]评论发送失败", zap.String("status", status), zap.String("msg", msg))
		return commentCreateResult{}
	}
	recordOutboundComment(source, text, link_id, reply_id, root_id, imageURL, data, commentID, createdAt)
	time.Sleep(5 * time.Second)
	return commentCreateResult{Sent: true, Handled: true}
}

func recordOutboundComment(source, text, linkIDText, replyIDText, rootIDText, imageURL string, response []byte, commentID int64, createdAt int64) {
	linkID := positiveInt64(linkIDText)
	replyID := positiveInt64(replyIDText)
	rootID := positiveInt64(rootIDText)
	if rootID <= 0 && replyID <= 0 && commentID > 0 {
		rootID = commentID
	}
	if createdAt <= 0 {
		createdAt = time.Now().Unix()
	}
	db.SaveOutboundMessage(db.OutboundMessage{
		Source:         source,
		LinkID:         linkID,
		RootCommentID:  rootID,
		ReplyCommentID: replyID,
		CommentID:      commentID,
		Text:           text,
		ImageURL:       imageURL,
		CreatedAt:      createdAt,
		RawResponse:    string(response),
	})
}

func parseCommentCreateResponse(data []byte) (string, string, int64, int64) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return "", "", 0, 0
	}
	return jsonString(payload["status"]), jsonString(payload["msg"]), findJSONInt(payload, "comment_id", "commentid", "commentId"), findJSONUnixTime(payload, "create_at", "created_at", "create_time", "created_time", "dateline", "publish_time")
}

func readableXHHResponseBody(body string) string {
	decoder := json.NewDecoder(bytes.NewReader([]byte(body)))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return limitXHHResponseBody(body)
	}
	decoded, err := json.Marshal(payload)
	if err != nil {
		return limitXHHResponseBody(body)
	}
	return limitXHHResponseBody(string(decoded))
}

func findJSONInt(value any, names ...string) int64 {
	switch typed := value.(type) {
	case map[string]any:
		for _, name := range names {
			if found, ok := typed[name]; ok {
				if number := jsonInt64(found); number > 0 {
					return number
				}
			}
		}
		for _, child := range typed {
			if number := findJSONInt(child, names...); number > 0 {
				return number
			}
		}
	case []any:
		for _, child := range typed {
			if number := findJSONInt(child, names...); number > 0 {
				return number
			}
		}
	}
	return 0
}

func findJSONUnixTime(value any, names ...string) int64 {
	switch typed := value.(type) {
	case map[string]any:
		for _, name := range names {
			if found, ok := typed[name]; ok {
				if unixTime := jsonUnixTime(found); unixTime > 0 {
					return unixTime
				}
			}
		}
		for _, child := range typed {
			if unixTime := findJSONUnixTime(child, names...); unixTime > 0 {
				return unixTime
			}
		}
	case []any:
		for _, child := range typed {
			if unixTime := findJSONUnixTime(child, names...); unixTime > 0 {
				return unixTime
			}
		}
	}
	return 0
}

func jsonUnixTime(value any) int64 {
	switch typed := value.(type) {
	case json.Number:
		if number, err := typed.Int64(); err == nil {
			return normalizeUnixTimestamp(number)
		}
		if number, err := strconv.ParseFloat(typed.String(), 64); err == nil {
			return normalizeUnixTimestamp(int64(number))
		}
	case float64:
		return normalizeUnixTimestamp(int64(typed))
	case string:
		return parseCommentTimeTextUnix(typed)
	}
	return 0
}

func jsonInt64(value any) int64 {
	switch typed := value.(type) {
	case json.Number:
		number, _ := typed.Int64()
		return number
	case float64:
		return int64(typed)
	case string:
		number, _ := strconv.ParseInt(typed, 10, 64)
		return number
	}
	return 0
}

func jsonString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func positiveInt64(value string) int64 {
	number, _ := strconv.ParseInt(value, 10, 64)
	if number > 0 {
		return number
	}
	return 0
}
