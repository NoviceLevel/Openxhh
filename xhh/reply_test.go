package xhh

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"openxhh/config"
	"openxhh/loger"
	"strings"
	"testing"

	"go.uber.org/zap"
)

type commentCreateHTTPResponse struct {
	statusCode int
	body       string
}

func setupCommentCreateHTTPTest(t *testing.T, statusCode int, body string) *[]string {
	return setupCommentCreateHTTPSequenceTest(t, []commentCreateHTTPResponse{{statusCode: statusCode, body: body}})
}

func setupCommentCreateHTTPSequenceTest(t *testing.T, responses []commentCreateHTTPResponse) *[]string {
	t.Helper()
	oldConfig := config.ConfigStruct
	oldInfo := Info
	oldLogger := loger.Loger
	oldLastSendReqTime := lastSendReqTime
	oldCooldownUntil := xhhCaptchaCooldownUntil.Load()
	oldCooldownLevel := xhhCaptchaCooldownLevel.Load()
	loger.Loger = zap.NewNop()
	requests := make([]string, 0, len(responses))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bbs/app/comment/create" {
			t.Fatalf("path = %q, want /bbs/app/comment/create", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		requests = append(requests, values.Get("text"))
		index := len(requests) - 1
		response := responses[len(responses)-1]
		if index < len(responses) {
			response = responses[index]
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(response.statusCode)
		_, _ = w.Write([]byte(response.body))
	}))
	config.ConfigStruct.Xhh.BaseUrl = server.URL
	config.ConfigStruct.Xhh.DeviceID = "test-device"
	config.ConfigStruct.Xhh.MinRequestInterval = 0
	Info.Cookie = "test-cookie"
	Info.HeyBoxId = "123"
	lastSendReqTime = oldLastSendReqTime
	xhhCaptchaCooldownUntil.Store(0)
	xhhCaptchaCooldownLevel.Store(0)
	t.Cleanup(func() {
		server.Close()
		config.ConfigStruct = oldConfig
		Info = oldInfo
		loger.Loger = oldLogger
		lastSendReqTime = oldLastSendReqTime
		xhhCaptchaCooldownUntil.Store(oldCooldownUntil)
		xhhCaptchaCooldownLevel.Store(oldCooldownLevel)
	})
	return &requests
}

func TestReplyPostReturnsFalseWhenXHHRejectsComment(t *testing.T) {
	setupCommentCreateHTTPTest(t, http.StatusOK, `{"status":"failed","msg":"您最近的评论频次异常，请稍后再试","result":{}}`)

	if ReplyPost("测试回复", "183102356") {
		t.Fatal("ReplyPost returned true for failed XHH response, want false")
	}
}

func TestCommentPostImageReturnsFalseWhenXHHRejectsComment(t *testing.T) {
	setupCommentCreateHTTPTest(t, http.StatusOK, `{"status":"failed","msg":"您最近的评论频次异常，请稍后再试","result":{}}`)

	if CommentPostImage("测试图片回复", "183102356", "https://example.com/a.png") {
		t.Fatal("CommentPostImage returned true for failed XHH response, want false")
	}
}

func TestReplyReturnsHandledWhenXHHRejectsComment(t *testing.T) {
	setupCommentCreateHTTPTest(t, http.StatusOK, `{"status":"failed","msg":"您最近的评论频次异常，请稍后再试","result":{}}`)

	if !Reply("测试回复", "183102356", "885370839", "885210606", "0") {
		t.Fatal("Reply returned false for handled failed response, want true to avoid duplicate retries")
	}
}

func TestReplyPostReturnsTrueOnlyWhenXHHAcceptsComment(t *testing.T) {
	setupCommentCreateHTTPTest(t, http.StatusOK, `{"status":"ok","msg":"","result":{"commentid":123,"created_at":1716350000}}`)

	if !ReplyPost("测试回复", "183102356") {
		t.Fatal("ReplyPost returned false for ok XHH response, want true")
	}
}

func TestRewriteXHHSensitiveCommentTextSoftensAccountSecurityWords(t *testing.T) {
	got := rewriteXHHSensitiveCommentText("Telegram 里验证码、手机号、邮箱和密码都别再给了")
	for _, unwanted := range []string{"Telegram", "验证码", "手机号", "密码"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("rewriteXHHSensitiveCommentText still contains %q in %q", unwanted, got)
		}
	}
	for _, want := range []string{"那个聊天软件", "那串验证数字", "绑定号码", "绑定邮箱", "登录口令"} {
		if !strings.Contains(got, want) {
			t.Fatalf("rewriteXHHSensitiveCommentText missing %q in %q", want, got)
		}
	}
}

func TestReplyPostRetriesBlockedWordResponseWithFixedFallback(t *testing.T) {
	requests := setupCommentCreateHTTPSequenceTest(t, []commentCreateHTTPResponse{
		{statusCode: http.StatusOK, body: `{"status":"failed","msg":"您的发言中带有违规屏蔽词，无法发送","result":{}}`},
		{statusCode: http.StatusOK, body: `{"status":"ok","msg":"","result":{"commentid":123,"created_at":1716350000}}`},
	})

	text := "Telegram 里去设置设备，把不认识的登录踢掉，然后改密码"
	if !ReplyPost(text, "183102356") {
		t.Fatal("ReplyPost returned false after blocked-word fallback succeeded, want true")
	}
	if len(*requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(*requests))
	}
	first := (*requests)[0]
	second := (*requests)[1]
	for _, unwanted := range []string{"Telegram", "验证码", "手机号", "密码"} {
		if strings.Contains(first, unwanted) {
			t.Fatalf("first request should be pre-rewritten and not contain %q: %q", unwanted, first)
		}
	}
	if second != "Manbaout" {
		t.Fatalf("fallback request = %q, want Manbaout", second)
	}
}
