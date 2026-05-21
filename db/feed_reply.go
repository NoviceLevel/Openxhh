package db

import (
	"context"

	"openxhh/loger"
	"openxhh/pg"
	"openxhh/sqlite"

	"go.uber.org/zap"
)

type FeedReplyRecord struct {
	LinkID    int64  `json:"linkId"`
	Title     string `json:"title"`
	AuthorID  int64  `json:"authorId"`
	Author    string `json:"author"`
	PostText  string `json:"postText"`
	ReplyText string `json:"replyText"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
	CreatedAt int64  `json:"createdAt"`
	RepliedAt int64  `json:"repliedAt"`
}

func migrateFeedReplyTable() {
	ctx := context.Background()
	if cfg.Type == "pg" {
		_, err := pg.Conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS feed_reply_records (
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
			loger.Loger.Warn("[DB]无法创建自动刷帖记录表", zap.Error(err))
		}
	}
	if cfg.Type == "sqlite" {
		_, err := sqlite.Db.Exec(`CREATE TABLE IF NOT EXISTS feed_reply_records (
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
			loger.Loger.Warn("[DB]无法创建自动刷帖记录表", zap.Error(err))
		}
	}
}

func FeedReplyRecordExists(linkID int64) bool {
	ctx := context.Background()
	var exists bool
	if cfg.Type == "pg" {
		err := pg.Conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM feed_reply_records WHERE link_id=$1)", linkID).Scan(&exists)
		if err != nil {
			loger.Loger.Warn("[DB]无法查询自动刷帖记录", zap.Error(err), zap.Int64("link_id", linkID))
		}
		return exists
	}
	if cfg.Type == "sqlite" {
		err := sqlite.Db.QueryRow("SELECT EXISTS(SELECT 1 FROM feed_reply_records WHERE link_id=?)", linkID).Scan(&exists)
		if err != nil {
			loger.Loger.Warn("[DB]无法查询自动刷帖记录", zap.Error(err), zap.Int64("link_id", linkID))
		}
	}
	return exists
}

func FeedReplyAttemptsSince(since int64) int {
	ctx := context.Background()
	var count int
	if cfg.Type == "pg" {
		err := pg.Conn.QueryRow(ctx, "SELECT COUNT(*) FROM feed_reply_records WHERE replied_at >= $1 AND status IN ('sent','dry_run')", since).Scan(&count)
		if err != nil {
			loger.Loger.Warn("[DB]无法统计自动刷帖次数", zap.Error(err))
		}
		return count
	}
	if cfg.Type == "sqlite" {
		err := sqlite.Db.QueryRow("SELECT COUNT(*) FROM feed_reply_records WHERE replied_at >= ? AND status IN ('sent','dry_run')", since).Scan(&count)
		if err != nil {
			loger.Loger.Warn("[DB]无法统计自动刷帖次数", zap.Error(err))
		}
	}
	return count
}

func SaveFeedReplyRecord(record FeedReplyRecord) bool {
	ctx := context.Background()
	if cfg.Type == "pg" {
		_, err := pg.Conn.Exec(ctx, `INSERT INTO feed_reply_records (link_id,title,author_id,author_name,post_text,reply_text,status,reason,created_at,replied_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT (link_id) DO UPDATE SET title=$2, author_id=$3, author_name=$4, post_text=$5, reply_text=$6, status=$7, reason=$8, created_at=$9, replied_at=$10`,
			record.LinkID, record.Title, record.AuthorID, record.Author, record.PostText, record.ReplyText, record.Status, record.Reason, record.CreatedAt, record.RepliedAt)
		if err != nil {
			loger.Loger.Warn("[DB]无法保存自动刷帖记录", zap.Error(err), zap.Int64("link_id", record.LinkID))
			return false
		}
		return true
	}
	if cfg.Type == "sqlite" {
		_, err := sqlite.Db.Exec(`INSERT INTO feed_reply_records (link_id,title,author_id,author_name,post_text,reply_text,status,reason,created_at,replied_at)
			VALUES (?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT (link_id) DO UPDATE SET title=excluded.title, author_id=excluded.author_id, author_name=excluded.author_name, post_text=excluded.post_text, reply_text=excluded.reply_text, status=excluded.status, reason=excluded.reason, created_at=excluded.created_at, replied_at=excluded.replied_at`,
			record.LinkID, record.Title, record.AuthorID, record.Author, record.PostText, record.ReplyText, record.Status, record.Reason, record.CreatedAt, record.RepliedAt)
		if err != nil {
			loger.Loger.Warn("[DB]无法保存自动刷帖记录", zap.Error(err), zap.Int64("link_id", record.LinkID))
			return false
		}
		return true
	}
	return false
}
