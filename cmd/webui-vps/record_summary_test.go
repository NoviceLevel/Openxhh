package main

import (
	"database/sql"
	"path/filepath"
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
