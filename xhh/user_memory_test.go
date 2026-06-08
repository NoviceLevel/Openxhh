package xhh

import (
	"database/sql"
	"openxhh/ai"
	"openxhh/config"
	"openxhh/db"
	"openxhh/loger"
	"openxhh/sqlite"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func setupSQLiteUserMemoryContextTest(t *testing.T) {
	t.Helper()
	oldType := config.ConfigStruct.DataBase.Type
	oldDB := sqlite.Db
	oldLogger := loger.Loger
	oldTimeForMemory := timeForMemory
	loger.Loger = zap.NewNop()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlite.Db = database
	config.ConfigStruct.DataBase.Type = "sqlite"
	timeForMemory = func() int64 { return 1234 }
	t.Cleanup(func() {
		database.Close()
		sqlite.Db = oldDB
		config.ConfigStruct.DataBase.Type = oldType
		loger.Loger = oldLogger
		timeForMemory = oldTimeForMemory
	})
	_, err = sqlite.Db.Exec(`CREATE TABLE user_memories (
		user_id BIGINT PRIMARY KEY,
		user_name TEXT DEFAULT '',
		summary TEXT DEFAULT '',
		interaction_count BIGINT DEFAULT 0,
		last_interaction_at BIGINT DEFAULT 0,
		updated_at BIGINT DEFAULT 0
	)`)
	if err != nil {
		t.Fatalf("create user_memories: %v", err)
	}
	_, err = sqlite.Db.Exec(`CREATE TABLE bot_memory_state (
		key TEXT PRIMARY KEY,
		value TEXT DEFAULT '',
		updated_at BIGINT DEFAULT 0
	)`)
	if err != nil {
		t.Fatalf("create bot_memory_state: %v", err)
	}
}

func TestAppendUserMemoryContextInjectsRelationshipAndMood(t *testing.T) {
	setupSQLiteUserMemoryContextTest(t)
	db.SaveUserInteraction(1001, "歌德", "我靠怎么又遇到你了真有缘啊", "又是你啊。", 10)
	db.SaveBotMood("刚遇到熟人式互动，心情有点新奇")

	contents := appendUserMemoryContext([]ai.Content{{Type: "text", Text: "楼层上下文"}}, 1001, "")
	if len(contents) != 2 {
		t.Fatalf("len(contents) = %d, want 2", len(contents))
	}
	got := contents[1].Text
	for _, want := range []string{"和当前用户的关系记忆", "歌德", "有缘", "短期心情", "不要生硬复述"} {
		if !strings.Contains(got, want) {
			t.Fatalf("memory context missing %q in %q", want, got)
		}
	}
}

func TestRememberSuccessfulReplyStoresMemoryAndMood(t *testing.T) {
	setupSQLiteUserMemoryContextTest(t)

	rememberSuccessfulReply(db.CommStruct{Uid: 1002, UserName: "烦恼777"}, "可爱捏", "哼，眼光不错。")
	mem, ok := db.GetUserMemory(1002)
	if !ok {
		t.Fatal("GetUserMemory returned false")
	}
	if mem.UserName != "烦恼777" || mem.InteractionCount != 1 || mem.LastInteractionAt != 1234 {
		t.Fatalf("memory = %+v", mem)
	}
	if !strings.Contains(mem.Summary, "可爱") {
		t.Fatalf("summary = %q", mem.Summary)
	}
	if mood := db.BotMood(); !strings.Contains(mood, "得意") {
		t.Fatalf("BotMood = %q, want 得意", mood)
	}
}
