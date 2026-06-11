package xhh

import (
	"openxhh/ai"
	"openxhh/config"
	"openxhh/loger"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestReplyQualityOnlyKeepsSendLevelChecks(t *testing.T) {
	restoreReplyQualityTestState(t)
	config.ConfigStruct.Ai.ChatName = "惠惠"

	styleReplies := []string{
		"建议你先看看预算和需求。",
		"这里是惠惠专席，要么领成就，要么报委托。",
		"*惠惠压低帽檐。*\n\n这事确实要先看清楚。\n\n*她又把法杖往地上一杵。*",
		"哼，这次算你说得不错🙂",
		"惠惠觉得这事可以先缓一下，惠惠不是不管你。",
		"转什么deepseek！本大魔法师又不是传送阵客服！",
	}
	for _, reply := range styleReplies {
		if got := aiReplyQualityIssue(reply); got != "" {
			t.Fatalf("aiReplyQualityIssue(%q) = %q, want no style rejection", reply, got)
		}
		if got := feedReplyQualityIssue(reply, "标题"); got != "" {
			t.Fatalf("feedReplyQualityIssue(%q) = %q, want no style rejection", reply, got)
		}
	}
}

func TestReplyQualityStillRejectsSkipAndOverLengthForSendSafety(t *testing.T) {
	restoreReplyQualityTestState(t)

	if got := aiReplyQualityIssue("SKIP"); got == "" {
		t.Fatal("aiReplyQualityIssue(SKIP) = empty, want send-level issue")
	}
	if got := feedReplyQualityIssue("SKIP", ""); got != "" {
		t.Fatalf("feedReplyQualityIssue(SKIP) = %q, want allowed feed skip", got)
	}

	tooLong := strings.Repeat("测", xhhCommentMaxRunes+1)
	if got := aiReplyQualityIssue(tooLong); got != "回复过长" {
		t.Fatalf("aiReplyQualityIssue over limit = %q, want 回复过长", got)
	}
	if got := feedReplyQualityIssue(tooLong, ""); got != "回复过长" {
		t.Fatalf("feedReplyQualityIssue over limit = %q, want 回复过长", got)
	}
}

func TestGenerateAIReplyDoesNotRetryStyleReplies(t *testing.T) {
	restoreReplyQualityTestState(t)

	calls := 0
	getAIReplyForQualityRetry = func([]ai.Content, string, []ai.Topics, []ai.Tags, ...zap.Field) string {
		calls++
		return "建议你先看看预算和需求。"
	}

	got, skipped := generateAIReplyWithQualityRetry(nil, "hello", nil, nil)
	if skipped {
		t.Fatal("generateAIReplyWithQualityRetry skipped style reply, want sent")
	}
	if got != "建议你先看看预算和需求。" {
		t.Fatalf("reply = %q", got)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestGenerateFeedReplyDoesNotRetryStyleReplies(t *testing.T) {
	restoreReplyQualityTestState(t)

	calls := 0
	getAIFeedReplyForQualityRetry = func(string, []ai.Content, string, []ai.Topics, []ai.Tags, ...zap.Field) string {
		calls++
		return "这价格看着还行，火力也不错，可以考虑。"
	}

	got := generateFeedReplyWithQualityRetry("prompt", nil, "instruction", "", nil, nil)
	if got != "这价格看着还行，火力也不错，可以考虑。" {
		t.Fatalf("reply = %q", got)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func restoreReplyQualityTestState(t *testing.T) {
	t.Helper()
	oldConfig := config.ConfigStruct
	oldLogger := loger.Loger
	oldAIReply := getAIReplyForQualityRetry
	oldFeedReply := getAIFeedReplyForQualityRetry
	loger.Loger = zap.NewNop()
	t.Cleanup(func() {
		config.ConfigStruct = oldConfig
		loger.Loger = oldLogger
		getAIReplyForQualityRetry = oldAIReply
		getAIFeedReplyForQualityRetry = oldFeedReply
	})
}
