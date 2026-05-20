package xhh

import (
	"database/sql"
	"openxhh/config"
	"openxhh/db"
	"openxhh/sqlite"
	"testing"
)

func resetReplySchedulerState(t *testing.T) {
	t.Helper()
	oldOwner := config.ConfigStruct.Xhh.Owner
	oldOwners := append([]int(nil), Owners...)
	oldOwnerIDsLoaded := ownerIDsLoaded
	oldMaxReplyThreads := MaxReplyThreads
	oldMaxPendingReplies := MaxPendingReplies
	t.Cleanup(func() {
		config.ConfigStruct.Xhh.Owner = oldOwner
		Owners = oldOwners
		ownerIDsLoaded = oldOwnerIDsLoaded
		MaxReplyThreads = oldMaxReplyThreads
		MaxPendingReplies = oldMaxPendingReplies
	})
}

func setupXHHSQLiteCommTest(t *testing.T) {
	t.Helper()
	oldType := config.ConfigStruct.DataBase.Type
	oldDB := sqlite.Db
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
}

func insertSchedulerCommForTest(t *testing.T, msgID, userID int) {
	t.Helper()
	_, err := sqlite.Db.Exec("INSERT INTO at (msg_id, comment_a_id, comment_root_id, link_id, user_a_id, user_a_name, comment_text, reply) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", msgID, msgID+1000, -1, 1, userID, "user", "text", false)
	if err != nil {
		t.Fatalf("insert comm: %v", err)
	}
}

func TestSelectReplyBatchLimitsNormalUsersToOne(t *testing.T) {
	resetReplySchedulerState(t)
	config.ConfigStruct.Xhh.Owner = "100"
	Owners = nil
	ownerIDsLoaded = false
	MaxReplyThreads = 3

	got := selectReplyBatch([]db.CommStruct{
		{MsgID: 1, Uid: 200},
		{MsgID: 2, Uid: 201},
		{MsgID: 3, Uid: 202},
	})

	if len(got) != 1 {
		t.Fatalf("len(selectReplyBatch) = %d, want 1", len(got))
	}
	if got[0].MsgID != 1 {
		t.Fatalf("selected MsgID = %d, want 1", got[0].MsgID)
	}
	if workers := replyWorkerCount(got); workers != 1 {
		t.Fatalf("replyWorkerCount = %d, want 1", workers)
	}
	if batchType := replyBatchType(got); batchType != "普通用户" {
		t.Fatalf("replyBatchType = %q, want 普通用户", batchType)
	}
}

func TestSelectReplyBatchUsesThreadLimitForOwners(t *testing.T) {
	resetReplySchedulerState(t)
	config.ConfigStruct.Xhh.Owner = "100"
	Owners = nil
	ownerIDsLoaded = false
	MaxReplyThreads = 2

	got := selectReplyBatch([]db.CommStruct{
		{MsgID: 1, Uid: 200},
		{MsgID: 2, Uid: 100},
		{MsgID: 3, Uid: 100},
		{MsgID: 4, Uid: 100},
	})

	if len(got) != 2 {
		t.Fatalf("len(selectReplyBatch) = %d, want 2", len(got))
	}
	for _, item := range got {
		if item.Uid != 100 {
			t.Fatalf("selected non-owner reply: %+v", item)
		}
	}
	if workers := replyWorkerCount(got); workers != 2 {
		t.Fatalf("replyWorkerCount = %d, want 2", workers)
	}
	if batchType := replyBatchType(got); batchType != "owner" {
		t.Fatalf("replyBatchType = %q, want owner", batchType)
	}
}

func TestReplyThreadLimitDefaultsWhenConfigInvalid(t *testing.T) {
	resetReplySchedulerState(t)
	MaxReplyThreads = 0

	if got := replyThreadLimit(); got != defaultMaxReplyThreads {
		t.Fatalf("replyThreadLimit = %d, want %d", got, defaultMaxReplyThreads)
	}
}

func TestNextReplyBatchPrefersOwnerOverOlderNormalReplies(t *testing.T) {
	resetReplySchedulerState(t)
	setupXHHSQLiteCommTest(t)
	config.ConfigStruct.Xhh.Owner = "100"
	Owners = nil
	ownerIDsLoaded = false
	MaxReplyThreads = 2
	insertSchedulerCommForTest(t, 10, 200)
	insertSchedulerCommForTest(t, 20, 201)
	insertSchedulerCommForTest(t, 30, 100)
	insertSchedulerCommForTest(t, 40, 100)
	insertSchedulerCommForTest(t, 50, 100)

	got := nextReplyBatch()
	if len(got) != 2 {
		t.Fatalf("len(nextReplyBatch) = %d, want 2", len(got))
	}
	for _, item := range got {
		if item.Uid != 100 {
			t.Fatalf("nextReplyBatch selected non-owner reply: %+v", item)
		}
	}
	if got[0].MsgID != 30 || got[1].MsgID != 40 {
		t.Fatalf("nextReplyBatch msg ids = [%d %d], want [30 40]", got[0].MsgID, got[1].MsgID)
	}
}

func TestNextReplyBatchFallsBackToSingleNormalReply(t *testing.T) {
	resetReplySchedulerState(t)
	setupXHHSQLiteCommTest(t)
	config.ConfigStruct.Xhh.Owner = "100"
	Owners = nil
	ownerIDsLoaded = false
	MaxReplyThreads = 3
	insertSchedulerCommForTest(t, 10, 200)
	insertSchedulerCommForTest(t, 20, 201)

	got := nextReplyBatch()
	if len(got) != 1 {
		t.Fatalf("len(nextReplyBatch) = %d, want 1", len(got))
	}
	if got[0].MsgID != 10 || got[0].Uid == 100 {
		t.Fatalf("nextReplyBatch selected %+v, want oldest normal reply", got[0])
	}
}

func TestOwnerIDsCachesEmptyResult(t *testing.T) {
	resetReplySchedulerState(t)
	config.ConfigStruct.Xhh.Owner = "bad"
	Owners = nil
	ownerIDsLoaded = false

	if got := ownerIDs(); len(got) != 0 {
		t.Fatalf("ownerIDs = %v, want empty", got)
	}
	config.ConfigStruct.Xhh.Owner = "100"
	if got := ownerIDs(); len(got) != 0 {
		t.Fatalf("ownerIDs after cached invalid config = %v, want empty", got)
	}
}
