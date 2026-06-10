package xhh

import (
	"net/http"
	"net/http/httptest"
	"openxhh/config"
	"openxhh/loger"
	"testing"

	"go.uber.org/zap"
)

func setupCommentCreateHTTPTest(t *testing.T, statusCode int, body string) {
	t.Helper()
	oldConfig := config.ConfigStruct
	oldInfo := Info
	oldLogger := loger.Loger
	oldLastSendReqTime := lastSendReqTime
	oldCooldownUntil := xhhCaptchaCooldownUntil.Load()
	oldCooldownLevel := xhhCaptchaCooldownLevel.Load()
	loger.Loger = zap.NewNop()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bbs/app/comment/create" {
			t.Fatalf("path = %q, want /bbs/app/comment/create", r.URL.Path)
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
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
