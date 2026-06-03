package db

import (
	"database/sql"
	"openxhh/config"
	"openxhh/loger"
	"openxhh/sqlite"
	"testing"

	"go.uber.org/zap"
)

func setupSQLiteFeedReplyTest(t *testing.T) {
	t.Helper()
	oldType := config.ConfigStruct.DataBase.Type
	oldDB := sqlite.Db
	oldLogger := loger.Loger
	loger.Loger = zap.NewNop()
	database, err := sql.Open("sqlite3", ":memory:")
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
	migrateFeedReplyTable()
}

func TestSaveFeedReplyRecordAndExists(t *testing.T) {
	setupSQLiteFeedReplyTest(t)
	record := FeedReplyRecord{
		LinkID:    1001,
		Title:     "测试帖子",
		AuthorID:  2002,
		Author:    "作者",
		PostText:  "正文",
		ReplyText: "回复",
		Status:    "dry_run",
		Reason:    "试运行未发送",
		CreatedAt: 10,
		RepliedAt: 20,
	}

	if !SaveFeedReplyRecord(record) {
		t.Fatal("SaveFeedReplyRecord returned false")
	}
	if !FeedReplyRecordExists(record.LinkID) {
		t.Fatal("FeedReplyRecordExists returned false")
	}
}

func TestFeedReplyAttemptsSinceCountsSentAndDryRun(t *testing.T) {
	setupSQLiteFeedReplyTest(t)
	records := []FeedReplyRecord{
		{LinkID: 1, Status: "dry_run", RepliedAt: 100},
		{LinkID: 2, Status: "sent", RepliedAt: 110},
		{LinkID: 3, Status: "skipped", RepliedAt: 120},
		{LinkID: 4, Status: "failed", RepliedAt: 130},
		{LinkID: 5, Status: "sent", RepliedAt: 10},
	}
	for _, record := range records {
		if !SaveFeedReplyRecord(record) {
			t.Fatalf("SaveFeedReplyRecord(%d) returned false", record.LinkID)
		}
	}

	if got := FeedReplyAttemptsSince(90); got != 2 {
		t.Fatalf("FeedReplyAttemptsSince = %d, want 2", got)
	}
}

func TestSaveFeedReplyRecordUpdatesExistingRow(t *testing.T) {
	setupSQLiteFeedReplyTest(t)
	if !SaveFeedReplyRecord(FeedReplyRecord{LinkID: 1, Status: "failed", Reason: "old", RepliedAt: 10}) {
		t.Fatal("initial SaveFeedReplyRecord returned false")
	}
	if !SaveFeedReplyRecord(FeedReplyRecord{LinkID: 1, Status: "dry_run", Reason: "new", RepliedAt: 20}) {
		t.Fatal("update SaveFeedReplyRecord returned false")
	}

	var status string
	var reason string
	if err := sqlite.Db.QueryRow("SELECT status, reason FROM feed_reply_records WHERE link_id=?", 1).Scan(&status, &reason); err != nil {
		t.Fatalf("query updated record: %v", err)
	}
	if status != "dry_run" || reason != "new" {
		t.Fatalf("updated row = (%q, %q), want (dry_run, new)", status, reason)
	}
}

func TestFeedReplyRecordUsesLinkIDAsDedupeKey(t *testing.T) {
	setupSQLiteFeedReplyTest(t)
	if !SaveFeedReplyRecord(FeedReplyRecord{LinkID: 42, Status: "sent", ReplyText: "first", RepliedAt: 100}) {
		t.Fatal("initial SaveFeedReplyRecord returned false")
	}
	if !SaveFeedReplyRecord(FeedReplyRecord{LinkID: 42, Status: "sent", ReplyText: "second", RepliedAt: 200}) {
		t.Fatal("second SaveFeedReplyRecord returned false")
	}

	var count int
	if err := sqlite.Db.QueryRow("SELECT COUNT(*) FROM feed_reply_records WHERE link_id=?", 42).Scan(&count); err != nil {
		t.Fatalf("count duplicate link_id rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("duplicate rows for same link_id = %d, want 1", count)
	}
	if !FeedReplyRecordExists(42) {
		t.Fatal("FeedReplyRecordExists returned false for saved link_id")
	}
}
