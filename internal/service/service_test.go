package service

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/W1ndys/easy-qfnu-kjs/internal/model"
	_ "modernc.org/sqlite"
)

func TestOpenAPIConfigMigrationRemovesAISettings(t *testing.T) {
	db := openMemoryDB(t)
	_, err := db.Exec(`
		CREATE TABLE api_config (
			id INTEGER PRIMARY KEY,
			ai_base_url TEXT NOT NULL DEFAULT '',
			ai_key TEXT NOT NULL DEFAULT '',
			ai_model TEXT NOT NULL DEFAULT '',
			ai_prompt_override TEXT NOT NULL DEFAULT '',
			open_api_enabled INTEGER NOT NULL DEFAULT 0,
			open_api_key TEXT NOT NULL DEFAULT '',
			updated_at DATETIME
		);
		INSERT INTO api_config (
			id, ai_base_url, ai_key, ai_model, ai_prompt_override,
			open_api_enabled, open_api_key
		) VALUES (1, 'https://example.com', 'ai-secret', 'model', 'prompt', 1, 'open-secret');
	`)
	if err != nil {
		t.Fatalf("创建旧配置表失败: %v", err)
	}

	service, err := NewOpenAPIConfigService(db)
	if err != nil {
		t.Fatalf("迁移开放接口配置失败: %v", err)
	}
	cfg, err := service.Get()
	if err != nil {
		t.Fatalf("读取开放接口配置失败: %v", err)
	}
	if !cfg.Enabled || cfg.APIKey != "open-secret" {
		t.Fatalf("开放接口配置未正确迁移: %+v", cfg)
	}
	requireUTCTimestamp(t, cfg.UpdatedAt)

	var legacyTableCount int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'api_config'
	`).Scan(&legacyTableCount); err != nil {
		t.Fatalf("检查旧配置表失败: %v", err)
	}
	if legacyTableCount != 0 {
		t.Fatal("旧 AI 配置表仍然存在")
	}
	if !service.ValidateKey("open-secret") || service.ValidateKey("wrong-secret") {
		t.Fatal("开放接口 Key 校验结果不正确")
	}

	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("序列化内部配置失败: %v", err)
	}
	if strings.Contains(string(encoded), "open-secret") {
		t.Fatal("内部配置序列化泄露了开放接口 Key")
	}
}

func TestAnnouncementTimesStoredAndReturnedAsUTC(t *testing.T) {
	db := openMemoryDB(t)
	service, err := NewAnnouncementService(db)
	if err != nil {
		t.Fatalf("初始化公告服务失败: %v", err)
	}

	announcement, err := service.Create(model.CreateAnnouncementRequest{
		Title:     "测试公告",
		Content:   "内容",
		Important: true,
	})
	if err != nil {
		t.Fatalf("创建公告失败: %v", err)
	}
	requireUTCTimestamp(t, announcement.CreatedAt)
	requireUTCTimestamp(t, announcement.UpdatedAt)

	var storedCreatedAt, storedUpdatedAt string
	if err := db.QueryRow(`
		SELECT created_at, updated_at FROM announcements WHERE id = ?
	`, announcement.ID).Scan(&storedCreatedAt, &storedUpdatedAt); err != nil {
		t.Fatalf("读取公告存储时间失败: %v", err)
	}
	requireUTCTimestamp(t, storedCreatedAt)
	requireUTCTimestamp(t, storedUpdatedAt)

	publicList, err := service.ListPublic()
	if err != nil {
		t.Fatalf("读取公开公告失败: %v", err)
	}
	if len(publicList) != 1 || publicList[0].CreatedAt != announcement.CreatedAt {
		t.Fatalf("公开公告 UTC 时间不正确: %+v", publicList)
	}
}

func TestQueryLogTimeStoredAsUTC(t *testing.T) {
	service, err := NewStatsService(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatalf("初始化统计服务失败: %v", err)
	}
	t.Cleanup(func() { _ = service.Close() })

	service.RecordQuery(model.QueryRecord{Keyword: "老文史楼"})
	var queriedAt string
	if err := service.DB().QueryRow(`SELECT queried_at FROM query_logs LIMIT 1`).Scan(&queriedAt); err != nil {
		t.Fatalf("读取查询日志时间失败: %v", err)
	}
	requireUTCTimestamp(t, queriedAt)
}

func openMemoryDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func requireUTCTimestamp(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t.Fatalf("时间 %q 不是 RFC 3339: %v", value, err)
	}
	if !strings.HasSuffix(value, "Z") || parsed.Location() != time.UTC {
		t.Fatalf("时间 %q 不是 UTC", value)
	}
	return parsed
}
