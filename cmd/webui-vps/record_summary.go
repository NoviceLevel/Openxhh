package main

import (
	"context"
	"database/sql"
	"errors"
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

func (s *serverState) readDatabaseRecordSummary(cfg appConfig, recentOnly bool, failedCount int) (recordSummary, error) {
	outbound, inbound, err := s.readMessageStreamRecords(cfg, recentOnly)
	if err != nil {
		return recordSummary{}, err
	}
	feedRecords, err := s.readFeedReplyRecords(cfg, recentOnly)
	if err != nil {
		return recordSummary{}, err
	}
	pending, err := s.readPendingReplyCount(cfg)
	if err != nil {
		return recordSummary{}, err
	}
	completed := len(outbound)
	for _, record := range feedRecords {
		if record.Status == "sent" || record.Status == "dry_run" {
			completed++
		}
	}
	return recordSummary{
		InteractionsCount: len(inbound),
		CompletedCount:    completed,
		FailedCount:       failedCount,
		Pending:           pending,
		RecordsCount:      len(outbound) + len(inbound) + len(feedRecords),
		HasDatabase:       true,
	}, nil
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

func isMissingTableError(err error) bool {
	if err == nil || errors.Is(err, sql.ErrNoRows) {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "no such table") || strings.Contains(text, "does not exist") || strings.Contains(text, "undefined_table")
}
