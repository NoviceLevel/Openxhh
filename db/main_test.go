package db

import (
	"database/sql"
	"errors"
	"openxhh/config"
	"openxhh/loger"
	"openxhh/sqlite"
	"testing"

	"go.uber.org/zap"
)

func setupSQLiteCommTest(t *testing.T) {
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
	_, err = sqlite.Db.Exec(`CREATE TABLE at (
		msg_id BIGINT PRIMARY KEY,
		comment_a_id BIGINT,
		comment_root_id BIGINT,
		link_id BIGINT,
		user_a_id BIGINT,
		user_a_name TEXT DEFAULT '',
		comment_text TEXT,
		reply boolean
	)`)
	if err != nil {
		t.Fatalf("create at table: %v", err)
	}
	migrateBlockedUserTable()
}

type failingCommScanner struct{}

func (failingCommScanner) Scan(dest ...any) error {
	return errors.New("scan failed")
}

func TestScanCommSkipsInvalidRow(t *testing.T) {
	oldLogger := loger.Loger
	loger.Loger = zap.NewNop()
	t.Cleanup(func() { loger.Loger = oldLogger })

	if got, ok := scanComm(failingCommScanner{}, "test"); ok || got != (CommStruct{}) {
		t.Fatalf("scanComm returned (%+v, %v), want zero value and false", got, ok)
	}
}

func insertCommForTest(t *testing.T, msgID, userID int, replied bool) {
	t.Helper()
	_, err := sqlite.Db.Exec("INSERT INTO at (msg_id, comment_a_id, comment_root_id, link_id, user_a_id, user_a_name, comment_text, reply) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", msgID, msgID+1000, -1, 1, userID, "user", "text", replied)
	if err != nil {
		t.Fatalf("insert comm: %v", err)
	}
}

func TestGetCommOrdersPendingByMessageID(t *testing.T) {
	setupSQLiteCommTest(t)
	insertCommForTest(t, 30, 200, false)
	insertCommForTest(t, 10, 201, false)
	insertCommForTest(t, 20, 202, true)

	got := GetComm(2)
	if len(got) != 2 {
		t.Fatalf("len(GetComm) = %d, want 2", len(got))
	}
	if got[0].MsgID != 10 || got[1].MsgID != 30 {
		t.Fatalf("GetComm msg ids = [%d %d], want [10 30]", got[0].MsgID, got[1].MsgID)
	}
}

func TestGetCommByUserIDsReturnsOnlyOwners(t *testing.T) {
	setupSQLiteCommTest(t)
	insertCommForTest(t, 10, 200, false)
	insertCommForTest(t, 20, 100, false)
	insertCommForTest(t, 30, 100, false)
	insertCommForTest(t, 40, 100, false)

	got := GetCommByUserIDs([]int{100}, 2)
	if len(got) != 2 {
		t.Fatalf("len(GetCommByUserIDs) = %d, want 2", len(got))
	}
	if got[0].Uid != 100 || got[1].Uid != 100 {
		t.Fatalf("GetCommByUserIDs returned non-owner rows: %+v", got)
	}
	if got[0].MsgID != 20 || got[1].MsgID != 30 {
		t.Fatalf("GetCommByUserIDs msg ids = [%d %d], want [20 30]", got[0].MsgID, got[1].MsgID)
	}
}

func TestGetCommExcludingUserIDsSkipsOwners(t *testing.T) {
	setupSQLiteCommTest(t)
	insertCommForTest(t, 10, 100, false)
	insertCommForTest(t, 20, 200, false)
	insertCommForTest(t, 30, 201, false)

	got := GetCommExcludingUserIDs([]int{100}, 1)
	if len(got) != 1 {
		t.Fatalf("len(GetCommExcludingUserIDs) = %d, want 1", len(got))
	}
	if got[0].Uid == 100 || got[0].MsgID != 20 {
		t.Fatalf("GetCommExcludingUserIDs returned %+v, want msg 20 non-owner", got[0])
	}
}

func TestGetCommByUserIDsWithoutLimitReturnsAllOwners(t *testing.T) {
	setupSQLiteCommTest(t)
	insertCommForTest(t, 10, 100, false)
	insertCommForTest(t, 20, 100, false)
	insertCommForTest(t, 30, 100, false)
	insertCommForTest(t, 40, 200, false)

	got := GetCommByUserIDs([]int{100}, 0)
	if len(got) != 3 {
		t.Fatalf("len(GetCommByUserIDs) = %d, want 3", len(got))
	}
	if got[0].MsgID != 10 || got[1].MsgID != 20 || got[2].MsgID != 30 {
		t.Fatalf("GetCommByUserIDs msg ids = [%d %d %d], want [10 20 30]", got[0].MsgID, got[1].MsgID, got[2].MsgID)
	}
}

func TestInsertWithUserNameReturnsFalseForDuplicateComment(t *testing.T) {
	setupSQLiteCommTest(t)

	if !InsertWithUserName(100, 200, -1, 1, 300, "user", "first", false) {
		t.Fatal("first InsertWithUserName returned false, want true")
	}
	if InsertWithUserName(101, 200, -1, 1, 300, "user", "duplicate comment", false) {
		t.Fatal("duplicate comment InsertWithUserName returned true, want false")
	}
	if InsertWithUserName(100, 201, -1, 1, 300, "user", "duplicate msg", false) {
		t.Fatal("duplicate msg InsertWithUserName returned true, want false")
	}
}

func TestBlockedUserLifecycle(t *testing.T) {
	setupSQLiteCommTest(t)
	insertCommForTest(t, 10, 12345, false)
	insertCommForTest(t, 20, 12345, false)
	insertCommForTest(t, 30, 67890, false)

	userID, userName, ok := UserByQueuedCommentID(1010)
	if !ok || userID != 12345 || userName != "user" {
		t.Fatalf("UserByQueuedCommentID = (%d,%q,%v), want (12345,user,true)", userID, userName, ok)
	}
	if IsBlockedUser(12345) {
		t.Fatal("IsBlockedUser returned true before saving")
	}
	if !SaveBlockedUser(12345, "user", "blocked by target") {
		t.Fatal("SaveBlockedUser returned false")
	}
	if !IsBlockedUser(12345) {
		t.Fatal("IsBlockedUser returned false after saving")
	}

	MarkBlockedUserRepliesHandled(12345)
	if got := PendingReplyCountByUser(12345); got != 0 {
		t.Fatalf("PendingReplyCountByUser(blocked) = %d, want 0", got)
	}
	if got := PendingReplyCountByUser(67890); got != 1 {
		t.Fatalf("PendingReplyCountByUser(other) = %d, want 1", got)
	}
}
