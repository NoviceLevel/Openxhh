package db

import (
	"context"
	"time"

	"openxhh/loger"
	"openxhh/pg"
	"openxhh/sqlite"

	"go.uber.org/zap"
)

func migrateBlockedUserTable() {
	ctx := context.Background()
	if cfg.Type == "pg" {
		_, err := pg.Conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS blocked_users (
			user_id BIGINT PRIMARY KEY,
			user_name TEXT DEFAULT '',
			reason TEXT DEFAULT '',
			created_at BIGINT DEFAULT 0,
			updated_at BIGINT DEFAULT 0
		)`)
		if err != nil {
			loger.Loger.Warn("[DB]无法创建本地屏蔽用户表", zap.Error(err))
		}
		return
	}
	if cfg.Type == "sqlite" {
		_, err := sqlite.Db.Exec(`CREATE TABLE IF NOT EXISTS blocked_users (
			user_id BIGINT PRIMARY KEY,
			user_name TEXT DEFAULT '',
			reason TEXT DEFAULT '',
			created_at BIGINT DEFAULT 0,
			updated_at BIGINT DEFAULT 0
		)`)
		if err != nil {
			loger.Loger.Warn("[DB]无法创建本地屏蔽用户表", zap.Error(err))
		}
	}
}

func SaveBlockedUser(userID int, userName, reason string) bool {
	if userID <= 0 {
		return false
	}
	now := time.Now().Unix()
	ctx := context.Background()
	if cfg.Type == "pg" {
		if pg.Conn == nil {
			return false
		}
		_, err := pg.Conn.Exec(ctx, `INSERT INTO blocked_users (user_id,user_name,reason,created_at,updated_at)
			VALUES ($1,$2,$3,$4,$4)
			ON CONFLICT (user_id) DO UPDATE SET
				user_name=CASE WHEN EXCLUDED.user_name <> '' THEN EXCLUDED.user_name ELSE blocked_users.user_name END,
				reason=CASE WHEN EXCLUDED.reason <> '' THEN EXCLUDED.reason ELSE blocked_users.reason END,
				updated_at=EXCLUDED.updated_at`, userID, userName, reason, now)
		if err != nil {
			loger.Loger.Warn("[DB]无法保存本地屏蔽用户", zap.Error(err), zap.Int("user_id", userID))
			return false
		}
		return true
	}
	if cfg.Type == "sqlite" {
		if sqlite.Db == nil {
			return false
		}
		_, err := sqlite.Db.Exec(`INSERT INTO blocked_users (user_id,user_name,reason,created_at,updated_at)
			VALUES (?,?,?,?,?)
			ON CONFLICT(user_id) DO UPDATE SET
				user_name=CASE WHEN excluded.user_name <> '' THEN excluded.user_name ELSE blocked_users.user_name END,
				reason=CASE WHEN excluded.reason <> '' THEN excluded.reason ELSE blocked_users.reason END,
				updated_at=excluded.updated_at`, userID, userName, reason, now, now)
		if err != nil {
			loger.Loger.Warn("[DB]无法保存本地屏蔽用户", zap.Error(err), zap.Int("user_id", userID))
			return false
		}
		return true
	}
	return false
}

func IsBlockedUser(userID int) bool {
	if userID <= 0 {
		return false
	}
	ctx := context.Background()
	var exists bool
	if cfg.Type == "pg" {
		if pg.Conn == nil {
			return false
		}
		err := pg.Conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM blocked_users WHERE user_id=$1)", userID).Scan(&exists)
		if err != nil {
			loger.Loger.Warn("[DB]无法查询本地屏蔽用户", zap.Error(err), zap.Int("user_id", userID))
			return false
		}
		return exists
	}
	if cfg.Type == "sqlite" {
		if sqlite.Db == nil {
			return false
		}
		err := sqlite.Db.QueryRow("SELECT EXISTS(SELECT 1 FROM blocked_users WHERE user_id=?)", userID).Scan(&exists)
		if err != nil {
			loger.Loger.Warn("[DB]无法查询本地屏蔽用户", zap.Error(err), zap.Int("user_id", userID))
			return false
		}
		return exists
	}
	return false
}

func UserByQueuedCommentID(commentID int) (userID int, userName string, ok bool) {
	if commentID <= 0 {
		return 0, "", false
	}
	ctx := context.Background()
	if cfg.Type == "pg" {
		if pg.Conn == nil {
			return 0, "", false
		}
		err := pg.Conn.QueryRow(ctx, "SELECT user_a_id, COALESCE(user_a_name,'') FROM at WHERE comment_a_id=$1 LIMIT 1", commentID).Scan(&userID, &userName)
		if err != nil {
			return 0, "", false
		}
		return userID, userName, userID > 0
	}
	if cfg.Type == "sqlite" {
		if sqlite.Db == nil {
			return 0, "", false
		}
		err := sqlite.Db.QueryRow("SELECT user_a_id, COALESCE(user_a_name,'') FROM at WHERE comment_a_id=? LIMIT 1", commentID).Scan(&userID, &userName)
		if err != nil {
			return 0, "", false
		}
		return userID, userName, userID > 0
	}
	return 0, "", false
}

func MarkBlockedUserRepliesHandled(userID int) {
	if userID <= 0 {
		return
	}
	ctx := context.Background()
	if cfg.Type == "pg" {
		if pg.Conn == nil {
			return
		}
		if _, err := pg.Conn.Exec(ctx, "UPDATE at SET reply=$1 WHERE user_a_id=$2 AND reply=false", true, userID); err != nil {
			loger.Loger.Warn("[DB]无法标记本地屏蔽用户待回复", zap.Error(err), zap.Int("user_id", userID))
		}
		return
	}
	if cfg.Type == "sqlite" {
		if sqlite.Db == nil {
			return
		}
		if _, err := sqlite.Db.Exec("UPDATE at SET reply=? WHERE user_a_id=? AND reply=false", true, userID); err != nil {
			loger.Loger.Warn("[DB]无法标记本地屏蔽用户待回复", zap.Error(err), zap.Int("user_id", userID))
		}
	}
}
