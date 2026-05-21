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

func TestLimitFeedTextCountsRunes(t *testing.T) {
	got := limitFeedText("猫猫猫", 2)
	if got != "猫猫" {
		t.Fatalf("limitFeedText = %q, want 猫猫", got)
	}
}
