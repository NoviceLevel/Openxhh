package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type recordSummary struct {
	InteractionsCount int  `json:"interactionsCount"`
	CompletedCount    int  `json:"completedCount"`
	FailedCount       int  `json:"failedCount"`
	Pending           int  `json:"pending"`
	RecordsCount      int  `json:"recordsCount"`
	HasDatabase       bool `json:"hasDatabase"`
}

type databaseRecordCounts struct {
	Inbound       int
	Outbound      int
	FeedCompleted int
	FeedRecords   int
}

func (s *serverState) readDatabaseRecordSummary(cfg appConfig, recentOnly bool, failedCount int) (recordSummary, error) {
	counts, err := s.readDatabaseRecordCounts(cfg, recentOnly)
	if err != nil {
		return recordSummary{}, err
	}
	pending, err := s.readPendingReplyCount(cfg)
	if err != nil {
		return recordSummary{}, err
	}
	return recordSummary{
		InteractionsCount: counts.Inbound,
		CompletedCount:    counts.Outbound + counts.FeedCompleted,
		FailedCount:       failedCount,
		Pending:           pending,
		RecordsCount:      counts.Outbound + counts.Inbound + counts.FeedRecords,
		HasDatabase:       true,
	}, nil
}

func (s *serverState) readDatabaseRecordCounts(cfg appConfig, recentOnly bool) (databaseRecordCounts, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.DataBase.Type)) {
	case "", "sqlite":
		return s.readSQLiteDatabaseRecordCounts(recentOnly)
	case "pg", "postgres", "postgresql":
		return readPostgresDatabaseRecordCounts(cfg, recentOnly)
	default:
		return databaseRecordCounts{}, fmt.Errorf("unsupported database type: %s", cfg.DataBase.Type)
	}
}

func (s *serverState) readSQLiteDatabaseRecordCounts(recentOnly bool) (databaseRecordCounts, error) {
	if _, err := os.Stat(filepath.Join(s.rootDir, "sql.db")); err != nil {
		return databaseRecordCounts{}, nil
	}
	database, err := s.openSQLiteDatabase()
	if err != nil {
		return databaseRecordCounts{}, err
	}
	defer database.Close()
	args := databaseRecordCountArgs(recentOnly)
	outbound, err := querySQLiteRecordCount(database, messageStreamCountQuery("outbound_messages", "?", recentOnly), args...)
	if err != nil {
		return databaseRecordCounts{}, err
	}
	inbound, err := querySQLiteRecordCount(database, messageStreamCountQuery("inbound_messages", "?", recentOnly), args...)
	if err != nil {
		return databaseRecordCounts{}, err
	}
	feedCompleted, err := querySQLiteRecordCount(database, feedReplyCompletedCountQuery("?", recentOnly), args...)
	if err != nil {
		return databaseRecordCounts{}, err
	}
	feedRecords, err := querySQLiteRecordCount(database, feedReplyRecordsCountQuery("?", recentOnly), args...)
	if err != nil {
		return databaseRecordCounts{}, err
	}
	return databaseRecordCounts{
		Inbound:       inbound,
		Outbound:      outbound,
		FeedCompleted: feedCompleted,
		FeedRecords:   feedRecords,
	}, nil
}

func readPostgresDatabaseRecordCounts(cfg appConfig, recentOnly bool) (databaseRecordCounts, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, postgresDSN(cfg))
	if err != nil {
		return databaseRecordCounts{}, err
	}
	defer pool.Close()
	args := databaseRecordCountArgs(recentOnly)
	outbound, err := queryPostgresRecordCount(ctx, pool, messageStreamCountQuery("outbound_messages", "$1", recentOnly), args...)
	if err != nil {
		return databaseRecordCounts{}, err
	}
	inbound, err := queryPostgresRecordCount(ctx, pool, messageStreamCountQuery("inbound_messages", "$1", recentOnly), args...)
	if err != nil {
		return databaseRecordCounts{}, err
	}
	feedCompleted, err := queryPostgresRecordCount(ctx, pool, feedReplyCompletedCountQuery("$1", recentOnly), args...)
	if err != nil {
		return databaseRecordCounts{}, err
	}
	feedRecords, err := queryPostgresRecordCount(ctx, pool, feedReplyRecordsCountQuery("$1", recentOnly), args...)
	if err != nil {
		return databaseRecordCounts{}, err
	}
	return databaseRecordCounts{
		Inbound:       inbound,
		Outbound:      outbound,
		FeedCompleted: feedCompleted,
		FeedRecords:   feedRecords,
	}, nil
}

func databaseRecordCountArgs(recentOnly bool) []any {
	if !recentOnly {
		return nil
	}
	return []any{time.Now().Add(-24 * time.Hour).Unix()}
}

func messageStreamCountQuery(table string, placeholder string, recentOnly bool) string {
	query := "SELECT COUNT(*) FROM " + table
	if recentOnly {
		query += " WHERE created_at >= " + placeholder
	}
	return query
}

func feedReplyCompletedCountQuery(placeholder string, recentOnly bool) string {
	query := "SELECT COUNT(*) FROM feed_reply_records WHERE status IN ('sent','dry_run')"
	if recentOnly {
		query += " AND replied_at >= " + placeholder
	}
	return query
}

func feedReplyRecordsCountQuery(placeholder string, recentOnly bool) string {
	query := "SELECT COUNT(*) FROM feed_reply_records"
	if recentOnly {
		query += " WHERE replied_at >= " + placeholder
	}
	return query
}

func querySQLiteRecordCount(database *sql.DB, query string, args ...any) (int, error) {
	return scanRecordCount(database.QueryRow(query, args...))
}

func queryPostgresRecordCount(ctx context.Context, pool *pgxpool.Pool, query string, args ...any) (int, error) {
	return scanRecordCount(pool.QueryRow(ctx, query, args...))
}

type recordCountScanner interface {
	Scan(dest ...any) error
}

func scanRecordCount(row recordCountScanner) (int, error) {
	var count int64
	if err := row.Scan(&count); err != nil {
		if isMissingTableError(err) {
			return 0, nil
		}
		return 0, err
	}
	return int(count), nil
}

func (s *serverState) readPendingReplyCount(cfg appConfig) (int, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.DataBase.Type)) {
	case "", "sqlite":
		return s.readSQLitePendingReplyCount()
	case "pg", "postgres", "postgresql":
		return readPostgresPendingReplyCount(cfg)
	default:
		return 0, nil
	}
}

func (s *serverState) readSQLitePendingReplyCount() (int, error) {
	if _, err := os.Stat(filepath.Join(s.rootDir, "sql.db")); err != nil {
		return 0, nil
	}
	database, err := s.openSQLiteDatabase()
	if err != nil {
		return 0, err
	}
	defer database.Close()
	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM at WHERE reply=false").Scan(&count)
	if err != nil {
		if isMissingTableError(err) {
			return 0, nil
		}
		return 0, err
	}
	return count, nil
}

func readPostgresPendingReplyCount(cfg appConfig) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, postgresDSN(cfg))
	if err != nil {
		return 0, err
	}
	defer pool.Close()
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM at WHERE reply=false").Scan(&count)
	if err != nil {
		if isMissingTableError(err) {
			return 0, nil
		}
		return 0, err
	}
	return count, nil
}

func (s *serverState) readPendingReplies(cfg appConfig, limit int) ([]pendingReplyRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	switch strings.ToLower(strings.TrimSpace(cfg.DataBase.Type)) {
	case "", "sqlite":
		return s.readSQLitePendingReplies(limit)
	case "pg", "postgres", "postgresql":
		return readPostgresPendingReplies(cfg, limit)
	default:
		return nil, nil
	}
}

func (s *serverState) readSQLitePendingReplies(limit int) ([]pendingReplyRecord, error) {
	if _, err := os.Stat(filepath.Join(s.rootDir, "sql.db")); err != nil {
		return nil, nil
	}
	database, err := s.openSQLiteDatabase()
	if err != nil {
		return nil, err
	}
	defer database.Close()
	rows, err := database.Query(`SELECT msg_id,comment_a_id,comment_root_id,link_id,user_a_id,COALESCE(user_a_name,''),COALESCE(comment_text,'')
		FROM at WHERE reply=false ORDER BY msg_id ASC LIMIT ?`, limit)
	if err != nil {
		if isMissingTableError(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var records []pendingReplyRecord
	for rows.Next() {
		var record pendingReplyRecord
		if err := rows.Scan(&record.MsgID, &record.CommentID, &record.RootCommentID, &record.LinkID, &record.UserID, &record.UserName, &record.Text); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func readPostgresPendingReplies(cfg appConfig, limit int) ([]pendingReplyRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, postgresDSN(cfg))
	if err != nil {
		return nil, err
	}
	defer pool.Close()
	rows, err := pool.Query(ctx, `SELECT msg_id,comment_a_id,comment_root_id,link_id,user_a_id,COALESCE(user_a_name,''),COALESCE(comment_text,'')
		FROM at WHERE reply=false ORDER BY msg_id ASC LIMIT $1`, limit)
	if err != nil {
		if isMissingTableError(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var records []pendingReplyRecord
	for rows.Next() {
		var record pendingReplyRecord
		if err := rows.Scan(&record.MsgID, &record.CommentID, &record.RootCommentID, &record.LinkID, &record.UserID, &record.UserName, &record.Text); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func isMissingTableError(err error) bool {
	if err == nil || errors.Is(err, sql.ErrNoRows) {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "no such table") || strings.Contains(text, "does not exist") || strings.Contains(text, "undefined_table")
}
