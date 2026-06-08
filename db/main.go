package db

import (
	"context"
	"strings"

	"openxhh/config"
	"openxhh/loger"
	"openxhh/pg"
	"openxhh/sqlite"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

var cfg = &config.ConfigStruct.DataBase

func Init() {
	switch cfg.Type {
	case "pg":
		pg.InitPostgreSQL()
	case "sqlite":
		sqlite.Init()
	default:
		loger.Loger.Fatal("[DB]无效的数据库类型")
	}
	migrateAtTable()
	migrateFeedReplyTable()
	migrateMessageStreamTables()
	migrateCommentCacheTables()
	migrateUserMemoryTables()
}

func migrateAtTable() {
	if cfg.Type == "pg" {
		_, err := pg.Conn.Exec(context.Background(), "ALTER TABLE at ADD COLUMN IF NOT EXISTS user_a_name TEXT DEFAULT ''")
		if err != nil {
			loger.Loger.Warn("[DB]无法迁移 user_a_name", zap.Error(err))
		}
	}
	if cfg.Type == "sqlite" {
		_, err := sqlite.Db.Exec("ALTER TABLE at ADD COLUMN user_a_name TEXT DEFAULT ''")
		if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			loger.Loger.Warn("[DB]无法迁移 user_a_name", zap.Error(err))
		}
	}
}

func Insert(msg_id, comment_a_id, comment_root_id, link_id, user_a_id int, comment_text string, reply bool) bool {
	return InsertWithUserName(msg_id, comment_a_id, comment_root_id, link_id, user_a_id, "", comment_text, reply)
}

func InsertWithUserName(msg_id, comment_a_id, comment_root_id, link_id, user_a_id int, user_a_name, comment_text string, reply bool) bool {
	ctx := context.Background()
	if comment_a_id > 0 && CommentExists(comment_a_id) {
		return false
	}
	if cfg.Type == "pg" {
		result, err := pg.Conn.Exec(ctx, "INSERT INTO at (msg_id,comment_a_id,comment_root_id,link_id,user_a_id,user_a_name,comment_text,reply) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (msg_id) DO NOTHING", msg_id, comment_a_id, comment_root_id, link_id, user_a_id, user_a_name, comment_text, reply)
		if err != nil {
			loger.Loger.Info("[DB]PsqlError", zap.Error(err))
			return false
		}
		return result.RowsAffected() > 0
	}
	if cfg.Type == "sqlite" {
		result, err := sqlite.Db.Exec("INSERT INTO at (msg_id,comment_a_id,comment_root_id,link_id,user_a_id,user_a_name,comment_text,reply) VALUES (?,?,?,?,?,?,?,?) ON CONFLICT (msg_id) DO NOTHING", msg_id, comment_a_id, comment_root_id, link_id, user_a_id, user_a_name, comment_text, reply)
		if err != nil {
			loger.Loger.Info("[DB]SQLiteERROR", zap.Error(err))
			return false
		}
		rows, err := result.RowsAffected()
		if err != nil {
			loger.Loger.Info("[DB]SQLite rows affected error", zap.Error(err))
			return false
		}
		return rows > 0
	}
	return false
}

func CommentExists(commentID int) bool {
	ctx := context.Background()
	var exists bool
	if cfg.Type == "pg" {
		err := pg.Conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM at WHERE comment_a_id=$1)", commentID).Scan(&exists)
		if err != nil {
			loger.Loger.Info("[DB]PsqlError", zap.Error(err))
			return false
		}
		return exists
	}
	if cfg.Type == "sqlite" {
		err := sqlite.Db.QueryRow("SELECT EXISTS(SELECT 1 FROM at WHERE comment_a_id=?)", commentID).Scan(&exists)
		if err != nil {
			loger.Loger.Info("[DB]SQLiteERROR", zap.Error(err))
			return false
		}
		return exists
	}
	return false
}

func Replyed(comment_id int) {
	ctx := context.Background()
	if cfg.Type == "pg" {
		pg.Conn.Exec(ctx, "UPDATE at SET reply=$1 WHERE comment_a_id=$2", true, comment_id)
	}
	if cfg.Type == "sqlite" {
		sqlite.Db.Exec("UPDATE at SET reply=? WHERE comment_a_id=?", true, comment_id)
	}
}

func ReplyedMsg(msgID int) {
	ctx := context.Background()
	if cfg.Type == "pg" {
		pg.Conn.Exec(ctx, "UPDATE at SET reply=$1 WHERE msg_id=$2", true, msgID)
	}
	if cfg.Type == "sqlite" {
		sqlite.Db.Exec("UPDATE at SET reply=? WHERE msg_id=?", true, msgID)
	}
}

type CommStruct struct {
	MsgID     int
	LinkID    int
	CommentID int
	RootID    int
	Text      string
	Uid       int
	UserName  string
}

func PendingReplyCount() int {
	return PendingReplyCountByUser(0)
}

func PendingReplyCountByUser(userID int) int {
	ctx := context.Background()
	var count int
	if cfg.Type == "pg" {
		var err error
		if userID > 0 {
			err = pg.Conn.QueryRow(ctx, "SELECT COUNT(*) FROM at WHERE reply=false AND user_a_id=$1", userID).Scan(&count)
		} else {
			err = pg.Conn.QueryRow(ctx, "SELECT COUNT(*) FROM at WHERE reply=false").Scan(&count)
		}
		if err != nil {
			loger.Loger.Error("[DB]无法获取待回复数量", zap.Error(err))
		}
		return count
	}
	if cfg.Type == "sqlite" {
		var err error
		if userID > 0 {
			err = sqlite.Db.QueryRow("SELECT COUNT(*) FROM at WHERE reply=false AND user_a_id=?", userID).Scan(&count)
		} else {
			err = sqlite.Db.QueryRow("SELECT COUNT(*) FROM at WHERE reply=false").Scan(&count)
		}
		if err != nil {
			loger.Loger.Error("[DB]无法获取待回复数量", zap.Error(err))
		}
	}
	return count
}

func GetComm(limit int) (CommArr []CommStruct) {
	if limit <= 0 {
		limit = 1
	}
	ctx := context.Background()
	if cfg.Type == "pg" {
		row, err := pg.Conn.Query(ctx, "SELECT msg_id,link_id,comment_a_id,comment_root_id,comment_text,user_a_id,user_a_name FROM at WHERE reply=false ORDER BY msg_id ASC LIMIT $1", limit)
		if err != nil {
			loger.Loger.Error("[DB]无法获取评论信息", zap.Error(err))
			return
		}
		defer row.Close()
		for row.Next() {
			if Comm, ok := scanComm(row, "pg"); ok {
				CommArr = append(CommArr, Comm)
			}
		}
		logRowsErr(row, "pg")
		return
	}
	if cfg.Type == "sqlite" {
		row, err := sqlite.Db.Query("SELECT msg_id,link_id,comment_a_id,comment_root_id,comment_text,user_a_id,user_a_name FROM at WHERE reply=false ORDER BY msg_id ASC LIMIT ?", limit)
		if err != nil {
			loger.Loger.Error("[DB]无法获取评论信息", zap.Error(err))
			return
		}
		defer row.Close()
		for row.Next() {
			if Comm, ok := scanComm(row, "sqlite"); ok {
				CommArr = append(CommArr, Comm)
			}
		}
		logRowsErr(row, "sqlite")
	}

	return
}

func GetCommByUserIDs(userIDs []int, limit int) []CommStruct {
	return getCommByUserFilter(userIDs, limit, false)
}

func GetCommExcludingUserIDs(userIDs []int, limit int) []CommStruct {
	return getCommByUserFilter(userIDs, limit, true)
}

func getCommByUserFilter(userIDs []int, limit int, exclude bool) (CommArr []CommStruct) {
	if len(userIDs) == 0 {
		if exclude {
			return GetComm(limit)
		}
		return nil
	}
	ctx := context.Background()
	if cfg.Type == "pg" {
		ids := int64UserIDs(userIDs)
		condition := "user_a_id = ANY($1::bigint[])"
		if exclude {
			condition = "NOT (user_a_id = ANY($1::bigint[]))"
		}
		query := "SELECT msg_id,link_id,comment_a_id,comment_root_id,comment_text,user_a_id,user_a_name FROM at WHERE reply=false AND " + condition + " ORDER BY msg_id ASC"
		var row pgx.Rows
		var err error
		if limit > 0 {
			row, err = pg.Conn.Query(ctx, query+" LIMIT $2", ids, limit)
		} else {
			row, err = pg.Conn.Query(ctx, query, ids)
		}
		if err != nil {
			loger.Loger.Error("[DB]无法获取评论信息", zap.Error(err))
			return
		}
		defer row.Close()
		for row.Next() {
			if Comm, ok := scanComm(row, "pg"); ok {
				CommArr = append(CommArr, Comm)
			}
		}
		logRowsErr(row, "pg")
		return
	}
	if cfg.Type == "sqlite" {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(userIDs)), ",")
		condition := "user_a_id IN (" + placeholders + ")"
		if exclude {
			condition = "user_a_id NOT IN (" + placeholders + ")"
		}
		args := make([]any, 0, len(userIDs)+1)
		for _, id := range userIDs {
			args = append(args, id)
		}
		query := "SELECT msg_id,link_id,comment_a_id,comment_root_id,comment_text,user_a_id,user_a_name FROM at WHERE reply=false AND " + condition + " ORDER BY msg_id ASC"
		if limit > 0 {
			query += " LIMIT ?"
			args = append(args, limit)
		}
		row, err := sqlite.Db.Query(query, args...)
		if err != nil {
			loger.Loger.Error("[DB]无法获取评论信息", zap.Error(err))
			return
		}
		defer row.Close()
		for row.Next() {
			if Comm, ok := scanComm(row, "sqlite"); ok {
				CommArr = append(CommArr, Comm)
			}
		}
		logRowsErr(row, "sqlite")
	}
	return
}

func int64UserIDs(userIDs []int) []int64 {
	ids := make([]int64, 0, len(userIDs))
	for _, id := range userIDs {
		ids = append(ids, int64(id))
	}
	return ids
}

func IsNew() bool {
	ctx := context.Background()
	var num int
	if cfg.Type == "pg" {
		row := pg.Conn.QueryRow(ctx, "SELECT COUNT(*) FROM at")
		if err := row.Scan(&num); err != nil {
			loger.Loger.Warn("[DB]无法判断是否首次运行", zap.Error(err))
		}
	}
	if cfg.Type == "sqlite" {
		row := sqlite.Db.QueryRow("SELECT COUNT(*) FROM at")
		if err := row.Scan(&num); err != nil {
			loger.Loger.Warn("[DB]无法判断是否首次运行", zap.Error(err))
		}
	}
	if num > 0 {
		return false
	} else {
		return true
	}
}

type commRowScanner interface {
	Scan(dest ...any) error
}

type rowsWithErr interface {
	Err() error
}

func scanComm(row commRowScanner, source string) (CommStruct, bool) {
	var Comm CommStruct
	if err := row.Scan(&Comm.MsgID, &Comm.LinkID, &Comm.CommentID, &Comm.RootID, &Comm.Text, &Comm.Uid, &Comm.UserName); err != nil {
		loger.Loger.Warn("[DB]无法解析评论信息", zap.Error(err), zap.String("source", source))
		return CommStruct{}, false
	}
	return Comm, true
}

func logRowsErr(rows rowsWithErr, source string) {
	if err := rows.Err(); err != nil {
		loger.Loger.Warn("[DB]读取评论信息中断", zap.Error(err), zap.String("source", source))
	}
}
