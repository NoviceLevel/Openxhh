package main

import (
	"database/sql"
	"path/filepath"
	"strconv"
	"testing"
)

func TestReadSQLitePendingRepliesReturnsQueuedItems(t *testing.T) {
	root := t.TempDir()
	database, err := sql.Open("sqlite", filepath.Join(root, "sql.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()
	_, err = database.Exec(`CREATE TABLE at (
		msg_id INTEGER,
		comment_a_id INTEGER,
		comment_root_id INTEGER,
		link_id INTEGER,
		user_a_id INTEGER,
		user_a_name TEXT,
		comment_text TEXT,
		reply BOOLEAN
	)`)
	if err != nil {
		t.Fatalf("create at table: %v", err)
	}
	_, err = database.Exec(`INSERT INTO at
		(msg_id,comment_a_id,comment_root_id,link_id,user_a_id,user_a_name,comment_text,reply)
		VALUES
		(20,200,100,900,2,'用户二','第二条',false),
		(10,100,100,800,1,'用户一','第一条',false),
		(15,150,100,850,3,'已回复','不应返回',true)`)
	if err != nil {
		t.Fatalf("insert at rows: %v", err)
	}

	state := &serverState{rootDir: root}
	var cfg appConfig
	cfg.DataBase.Type = "sqlite"
	records, err := state.readPendingReplies(cfg, 100)
	if err != nil {
		t.Fatalf("readPendingReplies returned error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2", len(records))
	}
	if records[0].MsgID != 10 || records[0].Text != "第一条" || records[0].UserName != "用户一" {
		t.Fatalf("first record = %+v, want msg 10 pending row", records[0])
	}
	if records[1].MsgID != 20 || records[1].Text != "第二条" || records[1].UserName != "用户二" {
		t.Fatalf("second record = %+v, want msg 20 pending row", records[1])
	}
}

func TestReadDatabaseRecordSummaryCountsBeyondMessageStreamLimit(t *testing.T) {
	root := t.TempDir()
	database, err := sql.Open("sqlite", filepath.Join(root, "sql.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	_, err = database.Exec(`CREATE TABLE inbound_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source TEXT DEFAULT '',
		message_id BIGINT DEFAULT 0,
		link_id BIGINT DEFAULT 0,
		root_comment_id BIGINT DEFAULT 0,
		reply_comment_id BIGINT DEFAULT 0,
		comment_id BIGINT DEFAULT 0,
		user_id BIGINT DEFAULT 0,
		user_name TEXT DEFAULT '',
		text TEXT DEFAULT '',
		created_at BIGINT DEFAULT 0,
		raw_response TEXT DEFAULT '',
		unique_key TEXT UNIQUE
	)`)
	if err != nil {
		t.Fatalf("create inbound_messages: %v", err)
	}
	_, err = database.Exec(`CREATE TABLE outbound_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source TEXT DEFAULT '',
		link_id BIGINT DEFAULT 0,
		root_comment_id BIGINT DEFAULT 0,
		reply_comment_id BIGINT DEFAULT 0,
		comment_id BIGINT DEFAULT 0,
		text TEXT DEFAULT '',
		image_url TEXT DEFAULT '',
		created_at BIGINT DEFAULT 0,
		raw_response TEXT DEFAULT '',
		unique_key TEXT UNIQUE
	)`)
	if err != nil {
		t.Fatalf("create outbound_messages: %v", err)
	}
	_, err = database.Exec(`CREATE TABLE feed_reply_records (
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
	_, err = database.Exec(`CREATE TABLE at (
		msg_id INTEGER,
		reply BOOLEAN
	)`)
	if err != nil {
		t.Fatalf("create at table: %v", err)
	}

	tx, err := database.Begin()
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	inboundStmt, err := tx.Prepare(`INSERT INTO inbound_messages (message_id,text,created_at,unique_key) VALUES (?,?,?,?)`)
	if err != nil {
		t.Fatalf("prepare inbound insert: %v", err)
	}
	for i := 0; i < 1001; i++ {
		if _, err := inboundStmt.Exec(i+1, "question", 1000+i, "inbound-test-"+strconv.Itoa(i)); err != nil {
			t.Fatalf("insert inbound %d: %v", i, err)
		}
	}
	if err := inboundStmt.Close(); err != nil {
		t.Fatalf("close inbound statement: %v", err)
	}
	outboundStmt, err := tx.Prepare(`INSERT INTO outbound_messages (link_id,text,created_at,unique_key) VALUES (?,?,?,?)`)
	if err != nil {
		t.Fatalf("prepare outbound insert: %v", err)
	}
	for i := 0; i < 1001; i++ {
		if _, err := outboundStmt.Exec(i+1, "reply", 1000+i, "outbound-test-"+strconv.Itoa(i)); err != nil {
			t.Fatalf("insert outbound %d: %v", i, err)
		}
	}
	if err := outboundStmt.Close(); err != nil {
		t.Fatalf("close outbound statement: %v", err)
	}
	feedStmt, err := tx.Prepare(`INSERT INTO feed_reply_records (link_id,status,created_at,replied_at) VALUES (?,?,?,?)`)
	if err != nil {
		t.Fatalf("prepare feed insert: %v", err)
	}
	for i := 0; i < 305; i++ {
		status := "skipped"
		if i < 302 {
			status = "sent"
		}
		if _, err := feedStmt.Exec(i+1, status, 2000+i, 2000+i); err != nil {
			t.Fatalf("insert feed %d: %v", i, err)
		}
	}
	if err := feedStmt.Close(); err != nil {
		t.Fatalf("close feed statement: %v", err)
	}
	if _, err := tx.Exec(`INSERT INTO at (msg_id,reply) VALUES (1,false),(2,true)`); err != nil {
		t.Fatalf("insert pending rows: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit transaction: %v", err)
	}

	state := &serverState{rootDir: root}
	defer func() {
		if state.db != nil {
			_ = state.db.Close()
		}
	}()
	var cfg appConfig
	cfg.DataBase.Type = "sqlite"
	summary, err := state.readDatabaseRecordSummary(cfg, false, 7)
	if err != nil {
		t.Fatalf("readDatabaseRecordSummary returned error: %v", err)
	}
	if !summary.HasDatabase {
		t.Fatalf("summary.HasDatabase = false, want true")
	}
	if summary.InteractionsCount != 1001 {
		t.Fatalf("summary.InteractionsCount = %d, want 1001", summary.InteractionsCount)
	}
	if summary.CompletedCount != 1303 {
		t.Fatalf("summary.CompletedCount = %d, want 1303", summary.CompletedCount)
	}
	if summary.RecordsCount != 2307 {
		t.Fatalf("summary.RecordsCount = %d, want 2307", summary.RecordsCount)
	}
	if summary.Pending != 1 {
		t.Fatalf("summary.Pending = %d, want 1", summary.Pending)
	}
	if summary.FailedCount != 7 {
		t.Fatalf("summary.FailedCount = %d, want 7", summary.FailedCount)
	}

	outbound, inbound, err := state.readSQLiteMessageStreamRecords(false)
	if err != nil {
		t.Fatalf("readSQLiteMessageStreamRecords returned error: %v", err)
	}
	if len(inbound) != 1000 {
		t.Fatalf("len(inbound) = %d, want list limit 1000", len(inbound))
	}
	if len(outbound) != 1000 {
		t.Fatalf("len(outbound) = %d, want list limit 1000", len(outbound))
	}
}
