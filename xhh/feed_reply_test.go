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
		{name: "natural reply without explicit anchor", reply: "这价格看着还行，火力也不错，可以考虑。", want: ""},
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

func TestReplyQualityAllowsTavernLengthWithinXHHLimit(t *testing.T) {
	oldConfig := config.ConfigStruct
	t.Cleanup(func() {
		config.ConfigStruct = oldConfig
	})
	config.ConfigStruct.Ai.ChatName = "惠惠"

	longReply := strings.Repeat("这是一句带动作和情绪的酒馆回复。", 20)
	if len([]rune(longReply)) <= 120 {
		t.Fatalf("test reply length = %d, want above old limit", len([]rune(longReply)))
	}
	if got := aiReplyQualityIssue(longReply); got != "" {
		t.Fatalf("aiReplyQualityIssue = %q, want empty for tavern-length reply", got)
	}
	if got := feedReplyQualityIssue(longReply, ""); got != "" {
		t.Fatalf("feedReplyQualityIssue = %q, want empty for tavern-length feed reply", got)
	}

	tooLong := strings.Repeat("测", xhhCommentMaxRunes+1)
	if got := aiReplyQualityIssue(tooLong); got != "回复过长" {
		t.Fatalf("aiReplyQualityIssue over limit = %q, want 回复过长", got)
	}
	if got := feedReplyQualityIssue(tooLong, ""); got != "回复过长" {
		t.Fatalf("feedReplyQualityIssue over limit = %q, want 回复过长", got)
	}
}

func TestFeedReplyRetryInstructionKeepsFeedRepliesSubtle(t *testing.T) {
	got := feedReplyRetryInstruction("原始指令", "太像角色表演")
	for _, want := range []string{"原始指令", "太像角色表演", "酒馆式反应", "不需要刻意压成短评"} {
		if !strings.Contains(got, want) {
			t.Fatalf("feedReplyRetryInstruction missing %q in %q", want, got)
		}
	}
}

func TestBuildFeedReplyInstructionUsesTavernReplyStyle(t *testing.T) {
	got := buildFeedReplyInstruction(feedLink{
		Title:       "测试帖子",
		Description: "正文摘要",
	})
	for _, want := range []string{"符合上下文的评论", "普通回复一样的酒馆人设", "自然接话", "动作、停顿、情绪和角色反应", "测试帖子", "正文摘要"} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildFeedReplyInstruction missing %q in %q", want, got)
		}
	}
	for _, unwanted := range []string{"短评论", "普通路过网友", "角色味只轻轻露出"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("buildFeedReplyInstruction should not contain old feed wording %q: %q", unwanted, got)
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
	if got := feedReplyQualityIssue("爆裂一击就够了，本大人看了都摇头。", ""); got != "" {
		t.Fatalf("feedReplyQualityIssue should not reject natural wording without configured anchor = %q", got)
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
