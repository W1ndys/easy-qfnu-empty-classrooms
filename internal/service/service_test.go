package service

import (
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/W1ndys/easy-qfnu-kjs/internal/model"
)

func TestAnnouncementCreateReturnsUTCTimes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("创建 mock 数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	storedLocation := time.FixedZone("UTC+8", 8*60*60)
	storedCreatedAt := time.Date(2026, 7, 29, 16, 0, 0, 0, storedLocation)
	storedUpdatedAt := storedCreatedAt.Add(time.Minute)
	mock.ExpectQuery(`INSERT INTO announcements`).
		WithArgs("测试公告", "内容", true, utcTimeArgument{}).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectQuery(`SELECT id, title, content, important, created_at, updated_at`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "title", "content", "important", "created_at", "updated_at",
		}).AddRow(7, "测试公告", "内容", true, storedCreatedAt, storedUpdatedAt))

	announcement, err := NewAnnouncementService(db).Create(model.CreateAnnouncementRequest{
		Title:     "测试公告",
		Content:   "内容",
		Important: true,
	})
	if err != nil {
		t.Fatalf("创建公告失败: %v", err)
	}
	if announcement.CreatedAt.Location() != time.UTC || announcement.UpdatedAt.Location() != time.UTC {
		t.Fatalf("公告时间未转换为 UTC: %+v", announcement)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL 调用不符合预期: %v", err)
	}
}

func TestRecordQueryWritesUTCTime(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("创建 mock 数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectExec(`INSERT INTO query_logs`).
		WithArgs("老文史楼", 0, "01", "02", 3, "127.0.0.1", "ua", utcTimeArgument{}).
		WillReturnResult(sqlmock.NewResult(1, 1))

	NewStatsService(db).RecordQuery(model.QueryRecord{
		Keyword:     "老文史楼",
		StartNode:   "01",
		EndNode:     "02",
		ResultCount: 3,
		IP:          "127.0.0.1",
		UAHash:      "ua",
	})
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL 调用不符合预期: %v", err)
	}
}

func TestStartOfDayUsesClientTimezone(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 7, 29, 20, 30, 0, 0, location)
	start := startOfDay(now, location)
	want := time.Date(2026, 7, 28, 16, 0, 0, 0, time.UTC)
	if !start.Equal(want) {
		t.Fatalf("日期边界错误: got=%s want=%s", start, want)
	}
}

type utcTimeArgument struct{}

func (utcTimeArgument) Match(value driver.Value) bool {
	timestamp, ok := value.(time.Time)
	return ok && timestamp.Location() == time.UTC
}
