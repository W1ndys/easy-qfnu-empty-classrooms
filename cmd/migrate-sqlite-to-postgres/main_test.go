package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/W1ndys/easy-qfnu-kjs/internal/database"
)

func TestParseLegacyTimeWithExplicitTimezone(t *testing.T) {
	parsed, err := parseLegacyTime("2026-07-29T08:00:00Z", time.FixedZone("ignored", 8*60*60))
	if err != nil {
		t.Fatalf("解析时间失败: %v", err)
	}
	want := time.Date(2026, 7, 29, 8, 0, 0, 0, time.UTC)
	if !parsed.Equal(want) || parsed.Location() != time.UTC {
		t.Fatalf("显式时区时间转换错误: got=%s want=%s", parsed, want)
	}
}

func TestParseLegacyTimeUsesConfiguredTimezone(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	parsed, err := parseLegacyTime("2026-07-29 16:00:00", location)
	if err != nil {
		t.Fatalf("解析时间失败: %v", err)
	}
	want := time.Date(2026, 7, 29, 8, 0, 0, 0, time.UTC)
	if !parsed.Equal(want) || parsed.Location() != time.UTC {
		t.Fatalf("无时区时间转换错误: got=%s want=%s", parsed, want)
	}
}

func TestColumnOr(t *testing.T) {
	columns := map[string]bool{"keyword": true}
	if got := columnOr(columns, "keyword", "''"); got != "keyword" {
		t.Fatalf("已有字段选择错误: %s", got)
	}
	if got := columnOr(columns, "ip", "''"); got != "''" {
		t.Fatalf("缺失字段回退错误: %s", got)
	}
}

func TestSQLiteToPostgreSQLIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_MIGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("未设置 TEST_MIGRATION_DATABASE_URL")
	}

	source, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatalf("创建 SQLite 测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = source.Close() })
	if _, err := source.Exec(`
		CREATE TABLE query_logs (
			id INTEGER PRIMARY KEY,
			keyword TEXT NOT NULL,
			date_offset INTEGER NOT NULL,
			start_node TEXT NOT NULL,
			end_node TEXT NOT NULL,
			result_count INTEGER NOT NULL,
			ip TEXT NOT NULL,
			ua_hash TEXT NOT NULL,
			queried_at DATETIME NOT NULL
		);
		INSERT INTO query_logs VALUES (
			1, '老文史楼', 0, '01', '02', 3, '127.0.0.1', 'ua', '2026-07-29T08:00:00Z'
		);
		CREATE TABLE announcements (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			important INTEGER NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
		INSERT INTO announcements VALUES (
			3, '公告', '内容', 1, '2026-07-29 16:00:00', '2026-07-29 17:00:00'
		);
	`); err != nil {
		t.Fatalf("写入 SQLite 测试数据失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	target, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("连接 PostgreSQL 测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = target.Close() })
	if err := database.Migrate(ctx, target); err != nil {
		t.Fatalf("初始化 PostgreSQL schema 失败: %v", err)
	}

	result, err := migrate(
		ctx,
		source,
		target,
		time.UTC,
		time.FixedZone("UTC+8", 8*60*60),
		false,
	)
	if err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	if result.queryLogs != 1 || result.announcements != 1 {
		t.Fatalf("迁移计数错误: %+v", result)
	}

	var queriedAt, announcementCreatedAt time.Time
	if err := target.QueryRow(`SELECT queried_at FROM query_logs WHERE id = 1`).Scan(&queriedAt); err != nil {
		t.Fatalf("读取迁移查询日志失败: %v", err)
	}
	if err := target.QueryRow(`SELECT created_at FROM announcements WHERE id = 3`).Scan(&announcementCreatedAt); err != nil {
		t.Fatalf("读取迁移公告失败: %v", err)
	}
	want := time.Date(2026, 7, 29, 8, 0, 0, 0, time.UTC)
	if !queriedAt.Equal(want) || !announcementCreatedAt.Equal(want) {
		t.Fatalf("迁移 UTC 时间错误: query=%s announcement=%s want=%s", queriedAt, announcementCreatedAt, want)
	}

	var nextQueryLogID, nextAnnouncementID int64
	if err := target.QueryRow(`
		INSERT INTO query_logs (keyword) VALUES ('序列测试') RETURNING id
	`).Scan(&nextQueryLogID); err != nil {
		t.Fatalf("验证查询日志序列失败: %v", err)
	}
	if err := target.QueryRow(`
		INSERT INTO announcements (title, content) VALUES ('序列测试', '内容') RETURNING id
	`).Scan(&nextAnnouncementID); err != nil {
		t.Fatalf("验证公告序列失败: %v", err)
	}
	if nextQueryLogID != 2 || nextAnnouncementID != 4 {
		t.Fatalf("Identity 序列错误: query_logs=%d announcements=%d", nextQueryLogID, nextAnnouncementID)
	}
}
