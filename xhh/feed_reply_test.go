package xhh

import (
	"database/sql"
	"openxhh/config"
	"openxhh/db"
	"openxhh/loger"
	"openxhh/sqlite"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
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

func TestFeedReplyPersistedWaitUsesSavedLastRun(t *testing.T) {
	setupSQLiteFeedReplySchedulerTest(t)
	oldConfig := config.ConfigStruct
	t.Cleanup(func() {
		config.ConfigStruct = oldConfig
	})
	config.ConfigStruct.FeedReply.Enabled = true
	config.ConfigStruct.FeedReply.Interval = 900
	lastRunAt := time.Now().Unix() - 300
	if !db.SaveFeedReplyLastRunAt(lastRunAt) {
		t.Fatal("SaveFeedReplyLastRunAt returned false")
	}

	wait := feedReplyPersistedWait()
	if wait < 590*time.Second || wait > 610*time.Second {
		t.Fatalf("feedReplyPersistedWait = %v, want about 600s", wait)
	}
}

func setupSQLiteFeedReplySchedulerTest(t *testing.T) {
	t.Helper()
	oldType := config.ConfigStruct.DataBase.Type
	oldDB := sqlite.Db
	oldLogger := loger.Loger
	loger.Loger = zap.NewNop()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlite.Db = database
	config.ConfigStruct.DataBase.Type = "sqlite"
	t.Cleanup(func() {
		database.Close()
		sqlite.Db = oldDB
		config.ConfigStruct.DataBase.Type = oldType
		loger.Loger = oldLogger
	})
	_, err = sqlite.Db.Exec(`CREATE TABLE feed_reply_records (
		link_id BIGINT PRIMARY KEY,
		title TEXT DEFAULT '',
		author_id BIGINT DEFAULT 0,
		author_name TEXT DEFAULT '',
		post_text TEXT DEFAULT '',
		reply_text TEXT DEFAULT '',
		status TEXT DEFAULT '',
		reason TEXT DEFAULT '',
		created_at BIGINT DEFAULT 0,
		replied_at BIGINT DEFAULT 0
	)`)
	if err != nil {
		t.Fatalf("create feed_reply_records: %v", err)
	}
	_, err = sqlite.Db.Exec(`CREATE TABLE feed_reply_state (
		key TEXT PRIMARY KEY,
		value BIGINT DEFAULT 0,
		updated_at BIGINT DEFAULT 0
	)`)
	if err != nil {
		t.Fatalf("create feed_reply_state: %v", err)
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

func TestBuildFeedReplyInstructionOnlyFramesPostInput(t *testing.T) {
	got := buildFeedReplyInstruction(feedLink{
		Title:       "测试帖子",
		Description: "正文摘要",
	})
	for _, want := range []string{"公开评论", "SKIP", "测试帖子", "正文摘要"} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildFeedReplyInstruction missing %q in %q", want, got)
		}
	}
	for _, unwanted := range []string{"惠惠", "普通评论员", "中立路人", "红魔族式夸张", "专席", "报委托", "转职路线", "传送阵", "领成就", "卷轴", "1-2句"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("buildFeedReplyInstruction should not add style constraint %q: %q", unwanted, got)
		}
	}
}

func TestFeedReplyQualityKeepsShortNaturalReplies(t *testing.T) {
	for _, reply := range []string{
		"建议你先看看预算和需求。",
		"这价格看着还行，火力也不错。",
		"哼，这次算你说得不错。",
	} {
		if got := feedReplyQualityIssue(reply, ""); got != "" {
			t.Fatalf("feedReplyQualityIssue(%q) = %q, want no issue", reply, got)
		}
	}

	tooLong := strings.Repeat("测", xhhCommentMaxRunes+1)
	if got := feedReplyQualityIssue(tooLong, ""); got != "回复过长" {
		t.Fatalf("feedReplyQualityIssue over limit = %q, want 回复过长", got)
	}
}

func TestLimitFeedTextCountsRunes(t *testing.T) {
	got := limitFeedText("猫猫猫", 2)
	if got != "猫猫" {
		t.Fatalf("limitFeedText = %q, want 猫猫", got)
	}
}
