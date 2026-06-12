package db

import (
	"database/sql"
	"openxhh/config"
	"openxhh/loger"
	"openxhh/sqlite"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func setupSQLiteUserMemoryTest(t *testing.T) {
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
	migrateUserMemoryTables()
}

func TestSaveUserInteractionCreatesAndUpdatesMemory(t *testing.T) {
	setupSQLiteUserMemoryTest(t)

	if !SaveUserInteraction(1001, "歌德", "我靠怎么又遇到你了真有缘啊", "又是你啊，这也算命运的深红指引。", 10) {
		t.Fatal("SaveUserInteraction returned false")
	}
	mem, ok := GetUserMemory(1001)
	if !ok {
		t.Fatal("GetUserMemory returned false")
	}
	if mem.UserName != "歌德" || mem.InteractionCount != 1 || mem.LastInteractionAt != 10 {
		t.Fatalf("memory meta = %+v", mem)
	}
	for _, want := range []string{"歌德", "有缘", "熟络"} {
		if !strings.Contains(mem.Summary, want) {
			t.Fatalf("summary missing %q: %q", want, mem.Summary)
		}
	}

	if !SaveUserInteraction(1001, "歌德", "可爱捏", "哼，眼光不错。", 20) {
		t.Fatal("second SaveUserInteraction returned false")
	}
	mem, ok = GetUserMemory(1001)
	if !ok {
		t.Fatal("second GetUserMemory returned false")
	}
	if mem.InteractionCount != 2 || mem.LastInteractionAt != 20 {
		t.Fatalf("updated memory meta = %+v", mem)
	}
	if !strings.Contains(mem.Summary, "可爱") || len([]rune(mem.Summary)) > userMemorySummaryMaxRune {
		t.Fatalf("updated summary = %q", mem.Summary)
	}
}

func TestBotMoodPersists(t *testing.T) {
	setupSQLiteUserMemoryTest(t)

	if BotMood() != "" {
		t.Fatalf("initial BotMood = %q, want empty", BotMood())
	}
	if !SaveBotMood("刚被夸过，有点得意") {
		t.Fatal("SaveBotMood returned false")
	}
	if got := BotMood(); got != "刚被夸过，有点得意" {
		t.Fatalf("BotMood = %q", got)
	}
}

func TestUserInteractionToneRecognizesTransferRoleCommand(t *testing.T) {
	got := userInteractionTone("转阿库娅")
	for _, want := range []string{"转接梗", "角色口吻"} {
		if !strings.Contains(got, want) {
			t.Fatalf("transfer role tone missing %q: %q", want, got)
		}
	}
}

func TestUserInteractionToneDoesNotTreatEveryTurnAsTransfer(t *testing.T) {
	got := userInteractionTone("这个中转站看起来挺赚钱的")
	if strings.Contains(got, "转接梗") {
		t.Fatalf("ordinary text was classified as transfer joke: %q", got)
	}
}
