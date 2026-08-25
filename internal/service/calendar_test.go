package service

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestRefreshFailureKeepsPreviousCalendarSnapshot(t *testing.T) {
	oldTime := time.Date(2026, 8, 18, 0, 1, 0, 0, chinaLocation)
	service := &CalendarService{
		currentYearStr: "2025-2026-2",
		baseTime:       oldTime,
		baseWeek:       18,
		totalWeeks:     20,
		hasPermission:  true,
		client: doerFunc(func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "jsjy_query") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`<html><body>学期：2026-2027-1</body></html>`)),
					Header:     make(http.Header),
				}, nil
			}
			return nil, fmt.Errorf("week endpoint unavailable")
		}),
	}

	if err := service.Refresh(); err == nil {
		t.Fatal("周次请求失败时 Refresh 应返回错误")
	}
	if service.currentYearStr != "2025-2026-2" || service.baseWeek != 18 || service.totalWeeks != 20 || !service.baseTime.Equal(oldTime) || !service.hasPermission {
		t.Fatalf("刷新失败破坏旧快照: term=%s week=%d total=%d time=%s permission=%t",
			service.currentYearStr, service.baseWeek, service.totalWeeks, service.baseTime, service.hasPermission)
	}
}

func TestParseWeekProgressIncludesTotalWeeks(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "标准脚本",
			content: `$("#li_showWeek").html("<span>第18周</span>/20周");`,
		},
		{
			name:    "斜杠周围有空白",
			content: `$("#li_showWeek").html("<span>第18周</span> / 20周");`,
		},
		{
			name:    "全角斜杠",
			content: `$("#li_showWeek").html("<span>第18周</span>／20周");`,
		},
		{
			name:    "忽略其他位置的周次文本",
			content: `其他内容第3周/5周; $("#li_showWeek").html("<span>第18周</span>/20周");`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current, total, ok := parseWeekProgress(tt.content)
			if !ok || current != 18 || total != 20 {
				t.Fatalf("周次解析错误: current=%d total=%d ok=%t", current, total, ok)
			}
		})
	}
}

func TestGetDateInfoRejectsDateOutsideCurrentTerm(t *testing.T) {
	service := &CalendarService{
		currentYearStr: "2026-2027-1",
		baseTime:       time.Date(2026, 12, 28, 0, 1, 0, 0, chinaLocation),
		baseWeek:       20,
		totalWeeks:     20,
		now: func() time.Time {
			return time.Date(2026, 12, 28, 9, 0, 0, 0, chinaLocation)
		},
	}

	_, _, err := service.GetDateInfo(7)
	if !errors.Is(err, ErrDateOutsideCurrentTerm) {
		t.Fatalf("跨学期日期错误: got=%v want=%v", err, ErrDateOutsideCurrentTerm)
	}
}

func TestGetDateInfoUsesChinaBusinessDate(t *testing.T) {
	instant := time.Date(2026, 8, 24, 17, 0, 0, 0, time.UTC)
	service := &CalendarService{
		currentYearStr: "2026-2027-1",
		baseTime:       time.Date(2026, 8, 24, 0, 1, 0, 0, chinaLocation),
		baseWeek:       1,
		now:            func() time.Time { return instant },
	}

	info, date, err := service.GetDateInfo(0)
	if err != nil {
		t.Fatalf("计算日期失败: %v", err)
	}
	if date != "2026-08-25" {
		t.Fatalf("北京时间日期错误: got=%s want=2026-08-25", date)
	}
	if info.Xq != "2" {
		t.Fatalf("北京时间星期错误: got=%s want=2", info.Xq)
	}
	if info.Zc != "1" {
		t.Fatalf("教学周错误: got=%s want=1", info.Zc)
	}
}

func TestGetDateInfoUsesBaseAndTargetMondaysWhenRefreshIsStale(t *testing.T) {
	service := &CalendarService{
		currentYearStr: "2026-2027-1",
		baseTime:       time.Date(2026, 8, 24, 0, 1, 0, 0, chinaLocation),
		baseWeek:       1,
		now: func() time.Time {
			return time.Date(2026, 8, 31, 9, 0, 0, 0, chinaLocation)
		},
	}

	info, date, err := service.GetDateInfo(0)
	if err != nil {
		t.Fatalf("计算日期失败: %v", err)
	}
	if date != "2026-08-31" {
		t.Fatalf("目标日期错误: got=%s want=2026-08-31", date)
	}
	if info.Zc != "2" {
		t.Fatalf("刷新延迟后的教学周错误: got=%s want=2", info.Zc)
	}
}

func TestNextRefreshTimeUsesUpcomingChinaMidnight(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "午夜零点后仍执行当天刷新",
			now:  time.Date(2026, 8, 25, 0, 0, 30, 0, chinaLocation),
			want: time.Date(2026, 8, 25, 0, 1, 0, 0, chinaLocation),
		},
		{
			name: "当天刷新时间已过则调度次日",
			now:  time.Date(2026, 8, 24, 17, 0, 0, 0, time.UTC),
			want: time.Date(2026, 8, 26, 0, 1, 0, 0, chinaLocation),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextRefreshTime(tt.now)
			if !got.Equal(tt.want) || got.Location() != chinaLocation {
				t.Fatalf("刷新时间错误: got=%s want=%s", got, tt.want)
			}
		})
	}
}

func TestGetDateInfoKeepsUnknownWeekWhenRefreshHasNoBaseWeek(t *testing.T) {
	tests := []struct {
		name     string
		baseTime time.Time
	}{
		{name: "基准时间为空"},
		{
			name:     "基准时间存在但当前不在教学周",
			baseTime: time.Date(2026, 8, 18, 0, 1, 0, 0, chinaLocation),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &CalendarService{
				currentYearStr: "2026-2027-1",
				baseTime:       tt.baseTime,
				baseWeek:       0,
				now: func() time.Time {
					return time.Date(2026, 8, 25, 9, 0, 0, 0, chinaLocation)
				},
			}

			info, _, err := service.GetDateInfo(7)
			if err != nil {
				t.Fatalf("未知教学周不应返回跨学期错误: %v", err)
			}
			if info.Zc != "0" {
				t.Fatalf("未知教学周不应被推算: got=%s want=0", info.Zc)
			}
		})
	}
}
