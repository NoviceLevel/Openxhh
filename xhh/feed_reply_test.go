package xhh

import (
	"openxhh/config"
	"testing"
)

func TestFeedReplyDryRunDefaultsToTrue(t *testing.T) {
	oldDryRun := config.ConfigStruct.FeedReply.DryRun
	config.ConfigStruct.FeedReply.DryRun = nil
	t.Cleanup(func() {
		config.ConfigStruct.FeedReply.DryRun = oldDryRun
	})

	if !feedReplyDryRun() {
		t.Fatal("feedReplyDryRun should default to true")
	}
}

func TestFeedReplyLimitsDefaultWhenInvalid(t *testing.T) {
	oldInterval := config.ConfigStruct.FeedReply.Interval
	oldMaxPerRun := config.ConfigStruct.FeedReply.MaxPerRun
	oldMaxPerDay := config.ConfigStruct.FeedReply.MaxPerDay
	config.ConfigStruct.FeedReply.Interval = 0
	config.ConfigStruct.FeedReply.MaxPerRun = 0
	config.ConfigStruct.FeedReply.MaxPerDay = 0
	t.Cleanup(func() {
		config.ConfigStruct.FeedReply.Interval = oldInterval
		config.ConfigStruct.FeedReply.MaxPerRun = oldMaxPerRun
		config.ConfigStruct.FeedReply.MaxPerDay = oldMaxPerDay
	})

	if got := feedReplyInterval(); got != 900 {
		t.Fatalf("feedReplyInterval = %d, want 900", got)
	}
	if got := feedReplyMaxPerRun(); got != 1 {
		t.Fatalf("feedReplyMaxPerRun = %d, want 1", got)
	}
	if got := feedReplyMaxPerDay(); got != 10 {
		t.Fatalf("feedReplyMaxPerDay = %d, want 10", got)
	}
}

func TestShouldSkipFeedReply(t *testing.T) {
	tests := []string{"SKIP", " skip。", "跳过", "不回复！"}
	for _, text := range tests {
		if !shouldSkipFeedReply(text) {
			t.Fatalf("shouldSkipFeedReply(%q) = false, want true", text)
		}
	}
	if shouldSkipFeedReply("这游戏氛围挺有意思的") {
		t.Fatal("normal reply should not be skipped")
	}
}

func TestSanitizeFeedReply(t *testing.T) {
	got := sanitizeFeedReply("```\n这帖子信息量挺足，评论区也很热闹。\n```")
	want := "这帖子信息量挺足，评论区也很热闹。"
	if got != want {
		t.Fatalf("sanitizeFeedReply = %q, want %q", got, want)
	}
}

func TestFeedReplyQualityIssue(t *testing.T) {
	oldConfig := config.ConfigStruct
	t.Cleanup(func() {
		config.ConfigStruct = oldConfig
	})
	config.ConfigStruct.Ai.ChatName = "惠惠"
	config.ConfigStruct.Ai.Description = "红魔族 大魔法使 爆裂魔法 本大人"
	config.ConfigStruct.Ai.Personality = ""
	config.ConfigStruct.Ai.Scenario = ""
	config.ConfigStruct.Ai.FirstMessage = ""
	config.ConfigStruct.Ai.ExampleDialogs = ""
	config.ConfigStruct.Ai.PostHistoryInstructions = ""
	config.ConfigStruct.Ai.Prompt = ""
	config.ConfigStruct.FeedReply.Description = ""
	config.ConfigStruct.FeedReply.Personality = ""
	config.ConfigStruct.FeedReply.Scenario = ""
	config.ConfigStruct.FeedReply.FirstMessage = ""
	config.ConfigStruct.FeedReply.ExampleDialogs = ""
	config.ConfigStruct.FeedReply.PostHistoryInstructions = ""
	config.ConfigStruct.FeedReply.Prompt = ""

	tests := []struct {
		name  string
		reply string
		title string
		want  string
	}{
		{name: "valid role reply", reply: "这价格有点像把金币丢进无效咏唱里，本大人看了都摇头。", want: ""},
		{name: "generic fantasy reply", reply: "这价格看着还行，火力也不错，可以考虑。", want: "缺少当前人设锚点"},
		{name: "customer tone", reply: "建议你先看看预算和需求。", want: "客服腔或暴露 AI 身份"},
		{name: "repeat title", title: "求评价，不玻璃心。", reply: "求评价，不玻璃心。这个配置还可以。", want: "复述标题"},
		{name: "skip allowed", reply: "SKIP", want: ""},
	}
	for _, tt := range tests {
		if got := feedReplyQualityIssue(tt.reply, tt.title); got != tt.want {
			t.Fatalf("%s: feedReplyQualityIssue = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestFeedReplyQualityIssueUsesConfiguredPersonaAnchors(t *testing.T) {
	oldConfig := config.ConfigStruct
	t.Cleanup(func() {
		config.ConfigStruct = oldConfig
	})
	config.ConfigStruct.Ai.ChatName = "悠悠"
	config.ConfigStruct.Ai.Description = "悠悠是红魔族法师，孤独、害羞，很想交朋友。"
	config.ConfigStruct.Ai.Personality = ""
	config.ConfigStruct.Ai.Scenario = ""
	config.ConfigStruct.Ai.FirstMessage = ""
	config.ConfigStruct.Ai.ExampleDialogs = ""
	config.ConfigStruct.Ai.PostHistoryInstructions = ""
	config.ConfigStruct.Ai.Prompt = ""
	config.ConfigStruct.FeedReply.Description = ""
	config.ConfigStruct.FeedReply.Personality = ""
	config.ConfigStruct.FeedReply.Scenario = ""
	config.ConfigStruct.FeedReply.FirstMessage = ""
	config.ConfigStruct.FeedReply.ExampleDialogs = ""
	config.ConfigStruct.FeedReply.PostHistoryInstructions = ""
	config.ConfigStruct.FeedReply.Prompt = ""

	if got := feedReplyQualityIssue("悠悠觉得这个配置还可以，只是别太冲动。", ""); got != "" {
		t.Fatalf("feedReplyQualityIssue with configured anchor = %q, want empty", got)
	}
	if got := feedReplyQualityIssue("爆裂一击就够了，本大人看了都摇头。", ""); got != "缺少当前人设锚点" {
		t.Fatalf("feedReplyQualityIssue should not accept hardcoded Megumin anchors = %q", got)
	}
}

func TestFeedReplyQualityIssueSkipsAnchorCheckWithoutPersona(t *testing.T) {
	oldConfig := config.ConfigStruct
	t.Cleanup(func() {
		config.ConfigStruct = oldConfig
	})
	config.ConfigStruct.Ai.ChatName = ""
	config.ConfigStruct.Ai.Description = ""
	config.ConfigStruct.Ai.Personality = ""
	config.ConfigStruct.Ai.Scenario = ""
	config.ConfigStruct.Ai.FirstMessage = ""
	config.ConfigStruct.Ai.ExampleDialogs = ""
	config.ConfigStruct.Ai.PostHistoryInstructions = ""
	config.ConfigStruct.Ai.Prompt = ""
	config.ConfigStruct.FeedReply.Description = ""
	config.ConfigStruct.FeedReply.Personality = ""
	config.ConfigStruct.FeedReply.Scenario = ""
	config.ConfigStruct.FeedReply.FirstMessage = ""
	config.ConfigStruct.FeedReply.ExampleDialogs = ""
	config.ConfigStruct.FeedReply.PostHistoryInstructions = ""
	config.ConfigStruct.FeedReply.Prompt = ""

	if got := feedReplyQualityIssue("这价格看着还行，火力也不错，可以考虑。", ""); got != "" {
		t.Fatalf("feedReplyQualityIssue without persona = %q, want empty", got)
	}
}

func TestLimitFeedTextCountsRunes(t *testing.T) {
	got := limitFeedText("猫猫猫", 2)
	if got != "猫猫" {
		t.Fatalf("limitFeedText = %q, want 猫猫", got)
	}
}
