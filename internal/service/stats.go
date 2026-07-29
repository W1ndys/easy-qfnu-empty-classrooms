package service

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/W1ndys/easy-qfnu-kjs/internal/model"
	"github.com/W1ndys/easy-qfnu-kjs/pkg/logger"
)

// StatsService 管理 PostgreSQL 中的查询统计数据。
type StatsService struct {
	db *sql.DB
}

func NewStatsService(db *sql.DB) *StatsService {
	logger.Info("统计服务已初始化")
	return &StatsService{db: db}
}

// RecordQuery 记录一次搜索查询，时间统一以 UTC 写入 TIMESTAMPTZ。
func (s *StatsService) RecordQuery(record model.QueryRecord) {
	if record.Keyword == "" {
		return
	}
	_, err := s.db.Exec(`
		INSERT INTO query_logs (
			keyword, date_offset, start_node, end_node,
			result_count, ip, ua_hash, queried_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		record.Keyword,
		record.DateOffset,
		record.StartNode,
		record.EndNode,
		record.ResultCount,
		record.IP,
		record.UAHash,
		time.Now().UTC(),
	)
	if err != nil {
		logger.Warn("记录搜索查询失败: %v", err)
	}
}

// GetStats 获取查询统计数据，默认按 UTC+8 划分自然日、周和月。
func (s *StatsService) GetStats() (*model.StatsResponse, error) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Now()
	todayStart := startOfDay(now, location)
	weekStart := startOfWeek(now, location)
	monthStart := startOfMonth(now, location)

	resp := &model.StatsResponse{}
	if err := s.db.QueryRow(`
		SELECT
			COUNT(*) FILTER (WHERE queried_at >= $1),
			COUNT(*) FILTER (WHERE queried_at >= $2),
			COUNT(*) FILTER (WHERE queried_at >= $3)
		FROM query_logs
	`, todayStart, weekStart, monthStart).Scan(
		&resp.TodayCount,
		&resp.WeekCount,
		&resp.MonthCount,
	); err != nil {
		return nil, fmt.Errorf("查询统计计数失败: %w", err)
	}

	var err error
	if resp.TodayTop, err = s.topKeywordSince(todayStart); err != nil {
		return nil, fmt.Errorf("查询今日最热关键词失败: %w", err)
	}
	if resp.WeekTop, err = s.topKeywordSince(weekStart); err != nil {
		return nil, fmt.Errorf("查询本周最热关键词失败: %w", err)
	}
	if resp.MonthTop, err = s.topKeywordSince(monthStart); err != nil {
		return nil, fmt.Errorf("查询本月最热关键词失败: %w", err)
	}
	return resp, nil
}

func (s *StatsService) topKeywordSince(start time.Time) (string, error) {
	var keyword string
	err := s.db.QueryRow(`
		SELECT keyword
		FROM query_logs
		WHERE queried_at >= $1
		GROUP BY keyword
		ORDER BY COUNT(*) DESC, keyword ASC
		LIMIT 1
	`, start).Scan(&keyword)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return keyword, err
}

// GetTopQueries 获取结果非空的热门查询组合。
func (s *StatsService) GetTopQueries(limit int) ([]model.TopQueryItem, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := s.db.Query(`
		SELECT keyword, date_offset, start_node, end_node, COUNT(*) AS count
		FROM query_logs
		WHERE result_count > 0
		GROUP BY keyword, date_offset, start_node, end_node
		ORDER BY count DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("查询热门搜索组合失败: %w", err)
	}
	defer rows.Close()

	queries := make([]model.TopQueryItem, 0)
	for rows.Next() {
		var item model.TopQueryItem
		if err := rows.Scan(
			&item.Building,
			&item.DateOffset,
			&item.StartNode,
			&item.EndNode,
			&item.Count,
		); err != nil {
			return nil, fmt.Errorf("扫描热门搜索组合失败: %w", err)
		}
		queries = append(queries, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历热门搜索组合失败: %w", err)
	}
	return queries, nil
}

// GetDashboardData 获取数据大屏综合统计数据。
// tzOffsetMin 是客户端相对 UTC 的分钟偏移，例如 UTC+8 为 480。
func (s *StatsService) GetDashboardData(timeRange string, days, tzOffsetMin int) (*model.DashboardResponse, error) {
	location := time.FixedZone("client", tzOffsetMin*60)
	now := time.Now().UTC()
	localNow := now.In(location)
	daysBack := 0
	switch timeRange {
	case "today":
	case "week":
		daysBack = 6
	case "month":
		daysBack = 29
	case "custom":
		if days < 1 {
			days = 1
		}
		daysBack = days - 1
	default:
		return nil, fmt.Errorf("不支持的时间范围: %s", timeRange)
	}
	startLocal := localNow.AddDate(0, 0, -daysBack)
	startTime := time.Date(
		startLocal.Year(), startLocal.Month(), startLocal.Day(),
		0, 0, 0, 0, location,
	).UTC()

	resp := &model.DashboardResponse{}
	var overviewErr, trendErr, keywordErr, nodeErr, resultErr, hourlyErr error
	var wg sync.WaitGroup
	wg.Add(6)

	go func() {
		defer wg.Done()
		resp.Overview, overviewErr = s.getDashboardOverview(startTime, location)
	}()
	go func() {
		defer wg.Done()
		resp.Trend, trendErr = s.getDashboardTrend(timeRange, startTime, localNow, days, tzOffsetMin)
	}()
	go func() {
		defer wg.Done()
		resp.TopKeywords, keywordErr = s.getDashboardKeywords(startTime)
	}()
	go func() {
		defer wg.Done()
		resp.NodeDist, nodeErr = s.getDashboardNodeDist(startTime)
	}()
	go func() {
		defer wg.Done()
		resp.ResultStats, resultErr = s.getDashboardResultStats(startTime)
	}()
	go func() {
		defer wg.Done()
		resp.HourlyDist, hourlyErr = s.getDashboardHourlyDist(startTime, tzOffsetMin)
	}()

	wg.Wait()
	for _, err := range []error{overviewErr, trendErr, keywordErr, nodeErr, resultErr, hourlyErr} {
		if err != nil {
			return nil, err
		}
	}
	return resp, nil
}

func (s *StatsService) getDashboardOverview(startTime time.Time, location *time.Location) (model.DashboardOverview, error) {
	var overview model.DashboardOverview
	if err := s.db.QueryRow(`
		SELECT
			COUNT(*),
			COUNT(DISTINCT keyword),
			COALESCE(AVG(result_count), 0)::DOUBLE PRECISION,
			COALESCE(MAX(result_count), 0)
		FROM query_logs
		WHERE queried_at >= $1
	`, startTime).Scan(
		&overview.TotalCount,
		&overview.UniqueKeywords,
		&overview.AvgResultCount,
		&overview.MaxResultCount,
	); err != nil {
		return overview, fmt.Errorf("查询总览数据失败: %w", err)
	}

	if err := s.db.QueryRow(`
		SELECT
			COUNT(DISTINCT ip || '|' || ua_hash),
			COUNT(DISTINCT ip)
		FROM query_logs
		WHERE queried_at >= $1 AND ip <> ''
	`, startTime).Scan(&overview.UniqueVisitors, &overview.UniqueIPs); err != nil {
		return overview, fmt.Errorf("查询独立用户数失败: %w", err)
	}

	now := time.Now()
	if err := s.db.QueryRow(`
		SELECT
			COUNT(*) FILTER (WHERE queried_at >= $1),
			COUNT(*) FILTER (WHERE queried_at >= $2),
			COUNT(*) FILTER (WHERE queried_at >= $3)
		FROM query_logs
	`,
		startOfDay(now, location),
		startOfWeek(now, location),
		startOfMonth(now, location),
	).Scan(
		&overview.TodayCount,
		&overview.WeekCount,
		&overview.MonthCount,
	); err != nil {
		return overview, fmt.Errorf("查询周期统计失败: %w", err)
	}
	return overview, nil
}

func (s *StatsService) getDashboardTrend(
	timeRange string,
	startTime time.Time,
	localNow time.Time,
	days, tzOffsetMin int,
) ([]model.TrendPoint, error) {
	format := "YYYY-MM-DD"
	pointCount := 0
	if timeRange == "today" {
		format = "HH24"
		pointCount = 24
	} else {
		switch timeRange {
		case "week":
			pointCount = 7
		case "month":
			pointCount = 30
		case "custom":
			if days < 1 {
				days = 1
			}
			pointCount = days
		default:
			return nil, fmt.Errorf("不支持的时间范围: %s", timeRange)
		}
	}

	rows, err := s.db.Query(`
		SELECT
			TO_CHAR(
				(queried_at AT TIME ZONE 'UTC') + ($2::INTEGER * INTERVAL '1 minute'),
				$3
			) AS label,
			COUNT(*) AS count
		FROM query_logs
		WHERE queried_at >= $1
		GROUP BY label
		ORDER BY label
	`, startTime, tzOffsetMin, format)
	if err != nil {
		return nil, fmt.Errorf("查询趋势失败: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var label string
		var count int
		if err := rows.Scan(&label, &count); err != nil {
			return nil, fmt.Errorf("扫描趋势数据失败: %w", err)
		}
		counts[label] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历趋势数据失败: %w", err)
	}

	points := make([]model.TrendPoint, 0, pointCount)
	if timeRange == "today" {
		for hour := 0; hour < 24; hour++ {
			key := fmt.Sprintf("%02d", hour)
			points = append(points, model.TrendPoint{
				Label: key + ":00",
				Count: counts[key],
			})
		}
		return points, nil
	}

	for offset := pointCount - 1; offset >= 0; offset-- {
		date := localNow.AddDate(0, 0, -offset)
		points = append(points, model.TrendPoint{
			Label: date.Format("01-02"),
			Count: counts[date.Format("2006-01-02")],
		})
	}
	return points, nil
}

func (s *StatsService) getDashboardKeywords(startTime time.Time) ([]model.KeywordRankItem, error) {
	rows, err := s.db.Query(`
		SELECT keyword, COUNT(*) AS count
		FROM query_logs
		WHERE queried_at >= $1
		GROUP BY keyword
		ORDER BY count DESC
		LIMIT 10
	`, startTime)
	if err != nil {
		return nil, fmt.Errorf("查询搜索词排行失败: %w", err)
	}
	defer rows.Close()

	items := make([]model.KeywordRankItem, 0)
	for rows.Next() {
		var item model.KeywordRankItem
		if err := rows.Scan(&item.Keyword, &item.Count); err != nil {
			return nil, fmt.Errorf("扫描搜索词排行失败: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *StatsService) getDashboardNodeDist(startTime time.Time) ([]model.NodeDistItem, error) {
	rows, err := s.db.Query(`
		SELECT start_node || '-' || end_node AS node_range, COUNT(*) AS count
		FROM query_logs
		WHERE queried_at >= $1 AND start_node <> '' AND end_node <> ''
		GROUP BY node_range
		ORDER BY count DESC
		LIMIT 10
	`, startTime)
	if err != nil {
		return nil, fmt.Errorf("查询节次分布失败: %w", err)
	}
	defer rows.Close()

	items := make([]model.NodeDistItem, 0)
	for rows.Next() {
		var item model.NodeDistItem
		if err := rows.Scan(&item.Node, &item.Count); err != nil {
			return nil, fmt.Errorf("扫描节次分布失败: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *StatsService) getDashboardResultStats(startTime time.Time) (model.ResultStatsData, error) {
	var result model.ResultStatsData
	if err := s.db.QueryRow(`
		SELECT
			COALESCE(AVG(result_count), 0)::DOUBLE PRECISION,
			COALESCE(MAX(result_count), 0),
			COALESCE(MIN(result_count) FILTER (WHERE result_count > 0), 0),
			COUNT(*) FILTER (WHERE result_count = 0),
			COUNT(*) FILTER (WHERE result_count > 0)
		FROM query_logs
		WHERE queried_at >= $1
	`, startTime).Scan(
		&result.AvgCount,
		&result.MaxCount,
		&result.MinCount,
		&result.ZeroCount,
		&result.NonZeroCount,
	); err != nil {
		return result, fmt.Errorf("查询结果统计失败: %w", err)
	}

	ranges := []struct {
		label string
		min   int
		max   int
	}{
		{"0", 0, 0},
		{"1-5", 1, 5},
		{"6-10", 6, 10},
		{"11-20", 11, 20},
		{"21-50", 21, 50},
		{"50+", 51, 999999},
	}
	result.Distribution = make([]model.ResultDistItem, 0, len(ranges))
	for _, itemRange := range ranges {
		var count int
		if err := s.db.QueryRow(`
			SELECT COUNT(*)
			FROM query_logs
			WHERE queried_at >= $1 AND result_count BETWEEN $2 AND $3
		`, startTime, itemRange.min, itemRange.max).Scan(&count); err != nil {
			return result, fmt.Errorf("查询结果区间分布失败: %w", err)
		}
		result.Distribution = append(result.Distribution, model.ResultDistItem{
			Range: itemRange.label,
			Count: count,
		})
	}
	return result, nil
}

func (s *StatsService) getDashboardHourlyDist(startTime time.Time, tzOffsetMin int) ([]model.HourlyDistItem, error) {
	rows, err := s.db.Query(`
		SELECT
			EXTRACT(HOUR FROM (
				(queried_at AT TIME ZONE 'UTC') + ($2::INTEGER * INTERVAL '1 minute')
			))::INTEGER AS hour,
			COUNT(*) AS count
		FROM query_logs
		WHERE queried_at >= $1
		GROUP BY hour
		ORDER BY hour
	`, startTime, tzOffsetMin)
	if err != nil {
		return nil, fmt.Errorf("查询每小时分布失败: %w", err)
	}
	defer rows.Close()

	counts := make(map[int]int)
	for rows.Next() {
		var hour, count int
		if err := rows.Scan(&hour, &count); err != nil {
			return nil, fmt.Errorf("扫描每小时分布失败: %w", err)
		}
		counts[hour] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历每小时分布失败: %w", err)
	}

	items := make([]model.HourlyDistItem, 24)
	for hour := 0; hour < 24; hour++ {
		items[hour] = model.HourlyDistItem{Hour: hour, Count: counts[hour]}
	}
	return items, nil
}

func startOfDay(now time.Time, location *time.Location) time.Time {
	local := now.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location).UTC()
}

func startOfWeek(now time.Time, location *time.Location) time.Time {
	local := now.In(location)
	weekday := int(local.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := local.AddDate(0, 0, -(weekday - 1))
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, location).UTC()
}

func startOfMonth(now time.Time, location *time.Location) time.Time {
	local := now.In(location)
	return time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location).UTC()
}
