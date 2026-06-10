package xhh

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"openxhh/db"
	"openxhh/loger"
	"strconv"
	"strings"
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
	sendText := rewriteXHHSensitiveCommentText(text)
	if sendText != text {
		loger.Loger.Info("[XHH]评论发送前已改写可能触发屏蔽的词", zap.String("source", source), zap.String("link_id", link_id), zap.String("reply_id", reply_id), zap.String("root_id", root_id))
	}
	status, msg, commentID, createdAt, data, ok := sendCommentCreateRequest(Path, sendText, link_id, reply_id, root_id, iscy, imageURL)
	if !ok {
		return commentCreateResult{}
	}
	if status != "ok" && isXHHBlockedWordMessage(msg) {
		fallbackText := blockedWordFallbackComment()
		if fallbackText != sendText {
			loger.Loger.Warn("[XHH]评论触发屏蔽词，改用固定兜底回复", zap.String("source", source), zap.String("link_id", link_id), zap.String("reply_id", reply_id), zap.String("root_id", root_id))
			status, msg, commentID, createdAt, data, ok = sendCommentCreateRequest(Path, fallbackText, link_id, reply_id, root_id, iscy, imageURL)
			if !ok {
				return commentCreateResult{}
			}
			if status == "ok" {
				sendText = fallbackText
			}
		}
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
	recordOutboundComment(source, sendText, link_id, reply_id, root_id, imageURL, data, commentID, createdAt)
	time.Sleep(5 * time.Second)
	return commentCreateResult{Sent: true, Handled: true}
}

func sendCommentCreateRequest(path, text, linkID, replyID, rootID, iscy, imageURL string) (status string, msg string, commentID int64, createdAt int64, data []byte, ok bool) {
	body := CommentCreateFormData(text, linkID, replyID, rootID, iscy, imageURL).Encode()
	resp := SendReq("POST", path, bytes.NewReader([]byte(body)), "")
	if resp == nil {
		loger.Loger.Error("[XHH]链接发送失败了")
		return "", "", 0, 0, nil, false
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		loger.Loger.Error("[XHH]无法解析Body", zap.Error(err), zap.Int("status", resp.StatusCode), zap.String("link_id", linkID), zap.String("reply_id", replyID), zap.String("root_id", rootID), zap.Bool("has_image", imageURL != ""))
		return "", "", 0, 0, data, false
	}
	if !isHTTPSuccess(resp.StatusCode) {
		body := string(data)
		loger.Loger.Error("[XHH]评论发送 HTTP 失败", zap.Int("status", resp.StatusCode), zap.String("link_id", linkID), zap.String("reply_id", replyID), zap.String("root_id", rootID), zap.Bool("has_image", imageURL != ""), zap.String("body", readableXHHResponseBody(body)))
		handleXHHHTTPFailure("comment_create", resp.StatusCode, body, zap.String("link_id", linkID), zap.String("reply_id", replyID), zap.String("root_id", rootID), zap.Bool("has_image", imageURL != ""))
		return "", "", 0, 0, data, false
	}
	status, msg, commentID, createdAt = parseCommentCreateResponse(data)
	if status == "" {
		loger.Loger.Error("[XHH]无法反序列化", zap.String("body", readableXHHResponseBody(string(data))))
		return "", "", 0, 0, data, false
	}
	return status, msg, commentID, createdAt, data, true
}

func rewriteXHHSensitiveCommentText(text string) string {
	replacer := strings.NewReplacer(
		"Telegram", "那个聊天软件",
		"telegram", "那个聊天软件",
		"TELEGRAM", "那个聊天软件",
		"tg", "那个聊天软件",
		"TG", "那个聊天软件",
		"电报", "那个聊天软件",
		"验证码", "那串验证数字",
		"手机号码", "绑定号码",
		"手机号", "绑定号码",
		"邮箱", "绑定邮箱",
		"账号密码", "账户登录口令",
		"密码", "登录口令",
		"账号", "账户",
		"改密", "改登录口令",
		"盗号", "账户被拿走",
		"冻结", "先暂停相关功能",
	)
	rewritten := replacer.Replace(text)
	rewritten = strings.ReplaceAll(rewritten, "绑定号码给", "绑定号码也给")
	rewritten = strings.ReplaceAll(rewritten, "绑定邮箱给", "绑定邮箱也给")
	return rewritten
}

func blockedWordFallbackComment() string {
	return "Manbaout"
}

func rewriteXHHBlockedCommentText(text string) string {
	text = rewriteXHHSensitiveCommentText(text)
	replacer := strings.NewReplacer(
		"改登录口令", "换个暗号",
		"登录口令", "暗号",
		"陌生设备", "陌生记录",
		"设备", "记录",
		"不认识的", "陌生的",
		"陌生登录", "陌生进入",
		"登录", "进入",
		"验证", "确认",
		"数字", "信息",
		"口令", "暗号",
		"账户", "号",
		"找回", "申诉处理",
		"申诉", "找客服处理",
		"安全", "稳妥",
	)
	return replacer.Replace(text)
}

func isXHHBlockedWordMessage(msg string) bool {
	return strings.Contains(msg, "屏蔽词") || strings.Contains(strings.ToLower(msg), "blocked")
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
