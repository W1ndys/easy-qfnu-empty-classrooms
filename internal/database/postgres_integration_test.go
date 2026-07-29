package database_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/W1ndys/easy-qfnu-kjs/internal/database"
	"github.com/W1ndys/easy-qfnu-kjs/internal/model"
	"github.com/W1ndys/easy-qfnu-kjs/internal/service"
)

func TestPostgreSQLRuntimeQueries(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("未设置 TEST_DATABASE_URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("连接测试 PostgreSQL 失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatalf("执行迁移失败: %v", err)
	}
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatalf("重复执行迁移失败: %v", err)
	}

	var timestampType string
	if err := db.QueryRowContext(ctx, `
		SELECT data_type
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'query_logs'
		  AND column_name = 'queried_at'
	`).Scan(&timestampType); err != nil {
		t.Fatalf("读取时间字段类型失败: %v", err)
	}
	if timestampType != "timestamp with time zone" {
		t.Fatalf("queried_at 类型错误: %s", timestampType)
	}

	announcementService := service.NewAnnouncementService(db)
	announcement, err := announcementService.Create(model.CreateAnnouncementRequest{
		Title:   "PostgreSQL 集成测试",
		Content: "内容",
	})
	if err != nil {
		t.Fatalf("创建公告失败: %v", err)
	}
	if announcement.CreatedAt.Location() != time.UTC {
		t.Fatalf("公告时间不是 UTC: %s", announcement.CreatedAt)
	}

	statsService := service.NewStatsService(db)
	statsService.RecordQuery(model.QueryRecord{
		Keyword:     "老文史楼",
		StartNode:   "01",
		EndNode:     "02",
		ResultCount: 3,
		IP:          "127.0.0.1",
		UAHash:      "integration-test",
	})
	stats, err := statsService.GetStats()
	if err != nil {
		t.Fatalf("查询统计失败: %v", err)
	}
	if stats.TodayCount < 1 {
		t.Fatalf("查询统计未包含测试数据: %+v", stats)
	}
	if _, err := statsService.GetDashboardData("today", 0, 480); err != nil {
		t.Fatalf("查询数据大屏失败: %v", err)
	}
}
