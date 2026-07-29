package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/W1ndys/easy-qfnu-kjs/internal/database"
	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

type options struct {
	sqlitePath           string
	databaseURL          string
	queryLogTimezone     string
	announcementTimezone string
	allowNonEmpty        bool
}

type migrationResult struct {
	queryLogs     int
	announcements int
}

func main() {
	_ = godotenv.Load()

	var opts options
	flag.StringVar(&opts.sqlitePath, "sqlite", "data/stats.db", "旧 SQLite 数据库文件路径")
	flag.StringVar(&opts.databaseURL, "database-url", os.Getenv("DATABASE_URL"), "目标 PostgreSQL DATABASE_URL")
	flag.StringVar(
		&opts.queryLogTimezone,
		"query-log-timezone",
		envOrDefault("SQLITE_QUERY_LOG_TIMEZONE", "UTC"),
		"无时区查询日志的原始时区",
	)
	flag.StringVar(
		&opts.announcementTimezone,
		"announcement-timezone",
		envOrDefault("SQLITE_ANNOUNCEMENT_TIMEZONE", "UTC"),
		"无时区公告时间的原始时区",
	)
	flag.BoolVar(&opts.allowNonEmpty, "allow-non-empty", false, "允许向非空目标表幂等写入；默认拒绝")
	flag.Parse()

	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "迁移失败: %v\n", err)
		os.Exit(1)
	}
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func run(opts options) error {
	queryLogLocation, err := time.LoadLocation(opts.queryLogTimezone)
	if err != nil {
		return fmt.Errorf("查询日志时区 %q 无效: %w", opts.queryLogTimezone, err)
	}
	announcementLocation, err := time.LoadLocation(opts.announcementTimezone)
	if err != nil {
		return fmt.Errorf("公告时区 %q 无效: %w", opts.announcementTimezone, err)
	}

	absPath, err := filepath.Abs(opts.sqlitePath)
	if err != nil {
		return fmt.Errorf("解析 SQLite 路径失败: %w", err)
	}
	if info, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("读取 SQLite 文件失败: %w", err)
	} else if !info.Mode().IsRegular() {
		return fmt.Errorf("SQLite 路径不是普通文件: %s", absPath)
	}

	sourceURL := (&url.URL{Scheme: "file", Path: absPath}).String() + "?mode=ro"
	source, err := sql.Open("sqlite", sourceURL)
	if err != nil {
		return fmt.Errorf("打开 SQLite 失败: %w", err)
	}
	defer source.Close()
	source.SetMaxOpenConns(1)
	if err := source.Ping(); err != nil {
		return fmt.Errorf("连接 SQLite 失败: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	target, err := database.Open(ctx, opts.databaseURL)
	if err != nil {
		return err
	}
	defer target.Close()
	if err := database.Migrate(ctx, target); err != nil {
		return fmt.Errorf("初始化 PostgreSQL schema 失败: %w", err)
	}

	result, err := migrate(ctx, source, target, queryLogLocation, announcementLocation, opts.allowNonEmpty)
	if err != nil {
		return err
	}
	fmt.Printf(
		"迁移完成: query_logs=%d announcements=%d，所有时间已转换为 UTC\n",
		result.queryLogs,
		result.announcements,
	)
	return nil
}

func migrate(
	ctx context.Context,
	source, target *sql.DB,
	queryLogLocation, announcementLocation *time.Location,
	allowNonEmpty bool,
) (migrationResult, error) {
	var result migrationResult
	sourceTx, err := source.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return result, fmt.Errorf("开始 SQLite 只读事务失败: %w", err)
	}
	defer sourceTx.Rollback()

	targetTx, err := target.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("开始 PostgreSQL 事务失败: %w", err)
	}
	defer targetTx.Rollback()
	if _, err := targetTx.ExecContext(ctx, `LOCK TABLE query_logs, announcements IN EXCLUSIVE MODE`); err != nil {
		return result, fmt.Errorf("锁定 PostgreSQL 目标表失败: %w", err)
	}

	var existingQueryLogs, existingAnnouncements int
	if err := targetTx.QueryRowContext(ctx, `SELECT COUNT(*) FROM query_logs`).Scan(&existingQueryLogs); err != nil {
		return result, fmt.Errorf("统计 PostgreSQL 查询日志失败: %w", err)
	}
	if err := targetTx.QueryRowContext(ctx, `SELECT COUNT(*) FROM announcements`).Scan(&existingAnnouncements); err != nil {
		return result, fmt.Errorf("统计 PostgreSQL 公告失败: %w", err)
	}
	if !allowNonEmpty && (existingQueryLogs != 0 || existingAnnouncements != 0) {
		return result, fmt.Errorf(
			"目标表非空（query_logs=%d announcements=%d）；请使用空库，或明确传入 -allow-non-empty",
			existingQueryLogs,
			existingAnnouncements,
		)
	}

	result.queryLogs, err = migrateQueryLogs(ctx, sourceTx, targetTx, queryLogLocation, allowNonEmpty)
	if err != nil {
		return result, err
	}
	result.announcements, err = migrateAnnouncements(ctx, sourceTx, targetTx, announcementLocation, allowNonEmpty)
	if err != nil {
		return result, err
	}
	if err := resetSequences(ctx, targetTx); err != nil {
		return result, err
	}

	if !allowNonEmpty {
		var targetQueryLogs, targetAnnouncements int
		if err := targetTx.QueryRowContext(ctx, `SELECT COUNT(*) FROM query_logs`).Scan(&targetQueryLogs); err != nil {
			return result, fmt.Errorf("校验 PostgreSQL 查询日志失败: %w", err)
		}
		if err := targetTx.QueryRowContext(ctx, `SELECT COUNT(*) FROM announcements`).Scan(&targetAnnouncements); err != nil {
			return result, fmt.Errorf("校验 PostgreSQL 公告失败: %w", err)
		}
		if targetQueryLogs != result.queryLogs || targetAnnouncements != result.announcements {
			return result, fmt.Errorf(
				"迁移计数不一致: SQLite=(%d,%d) PostgreSQL=(%d,%d)",
				result.queryLogs,
				result.announcements,
				targetQueryLogs,
				targetAnnouncements,
			)
		}
	}

	if err := targetTx.Commit(); err != nil {
		return result, fmt.Errorf("提交 PostgreSQL 迁移失败: %w", err)
	}
	if err := sourceTx.Commit(); err != nil {
		return result, fmt.Errorf("结束 SQLite 只读事务失败: %w", err)
	}
	return result, nil
}

func migrateQueryLogs(
	ctx context.Context,
	source *sql.Tx,
	target *sql.Tx,
	legacyLocation *time.Location,
	upsert bool,
) (int, error) {
	exists, err := sqliteTableExists(ctx, source, "query_logs")
	if err != nil || !exists {
		return 0, err
	}
	columns, err := sqliteColumns(ctx, source, "query_logs")
	if err != nil {
		return 0, err
	}
	keywordColumn := "keyword"
	if !columns[keywordColumn] {
		if columns["classroom"] {
			keywordColumn = "classroom"
		} else {
			return 0, fmt.Errorf("SQLite query_logs 缺少 keyword/classroom 字段")
		}
	}
	selectSQL := fmt.Sprintf(`
		SELECT %s, %s, %s, %s, %s, %s, %s, %s, CAST(%s AS TEXT)
		FROM query_logs ORDER BY %s
	`,
		columnOr(columns, "id", "rowid"),
		keywordColumn,
		columnOr(columns, "date_offset", "0"),
		columnOr(columns, "start_node", "''"),
		columnOr(columns, "end_node", "''"),
		columnOr(columns, "result_count", "0"),
		columnOr(columns, "ip", "''"),
		columnOr(columns, "ua_hash", "''"),
		columnOr(columns, "queried_at", "CURRENT_TIMESTAMP"),
		columnOr(columns, "id", "rowid"),
	)
	rows, err := source.QueryContext(ctx, selectSQL)
	if err != nil {
		return 0, fmt.Errorf("读取 SQLite 查询日志失败: %w", err)
	}
	defer rows.Close()

	insertSQL := `
		INSERT INTO query_logs (
			id, keyword, date_offset, start_node, end_node,
			result_count, ip, ua_hash, queried_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	if upsert {
		insertSQL += ` ON CONFLICT (id) DO UPDATE SET
			keyword = EXCLUDED.keyword,
			date_offset = EXCLUDED.date_offset,
			start_node = EXCLUDED.start_node,
			end_node = EXCLUDED.end_node,
			result_count = EXCLUDED.result_count,
			ip = EXCLUDED.ip,
			ua_hash = EXCLUDED.ua_hash,
			queried_at = EXCLUDED.queried_at`
	}

	count := 0
	for rows.Next() {
		var id int64
		var keyword, startNode, endNode, ip, uaHash, rawTime string
		var dateOffset, resultCount int
		if err := rows.Scan(
			&id,
			&keyword,
			&dateOffset,
			&startNode,
			&endNode,
			&resultCount,
			&ip,
			&uaHash,
			&rawTime,
		); err != nil {
			return count, fmt.Errorf("扫描 SQLite 查询日志失败: %w", err)
		}
		queriedAt, err := parseLegacyTime(rawTime, legacyLocation)
		if err != nil {
			return count, fmt.Errorf("解析 query_logs.id=%d 时间失败: %w", id, err)
		}
		if _, err := target.ExecContext(
			ctx,
			insertSQL,
			id,
			keyword,
			dateOffset,
			startNode,
			endNode,
			resultCount,
			ip,
			uaHash,
			queriedAt,
		); err != nil {
			return count, fmt.Errorf("写入 PostgreSQL query_logs.id=%d 失败: %w", id, err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return count, fmt.Errorf("遍历 SQLite 查询日志失败: %w", err)
	}
	return count, nil
}

func migrateAnnouncements(
	ctx context.Context,
	source *sql.Tx,
	target *sql.Tx,
	legacyLocation *time.Location,
	upsert bool,
) (int, error) {
	exists, err := sqliteTableExists(ctx, source, "announcements")
	if err != nil || !exists {
		return 0, err
	}
	rows, err := source.QueryContext(ctx, `
		SELECT id, title, content, important,
			CAST(created_at AS TEXT), CAST(updated_at AS TEXT)
		FROM announcements ORDER BY id
	`)
	if err != nil {
		return 0, fmt.Errorf("读取 SQLite 公告失败: %w", err)
	}
	defer rows.Close()

	insertSQL := `
		INSERT INTO announcements (
			id, title, content, important, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6)
	`
	if upsert {
		insertSQL += ` ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			content = EXCLUDED.content,
			important = EXCLUDED.important,
			created_at = EXCLUDED.created_at,
			updated_at = EXCLUDED.updated_at`
	}

	count := 0
	for rows.Next() {
		var id int64
		var title, content, rawCreatedAt, rawUpdatedAt string
		var important int
		if err := rows.Scan(&id, &title, &content, &important, &rawCreatedAt, &rawUpdatedAt); err != nil {
			return count, fmt.Errorf("扫描 SQLite 公告失败: %w", err)
		}
		createdAt, err := parseLegacyTime(rawCreatedAt, legacyLocation)
		if err != nil {
			return count, fmt.Errorf("解析 announcements.id=%d 创建时间失败: %w", id, err)
		}
		updatedAt, err := parseLegacyTime(rawUpdatedAt, legacyLocation)
		if err != nil {
			return count, fmt.Errorf("解析 announcements.id=%d 更新时间失败: %w", id, err)
		}
		if _, err := target.ExecContext(
			ctx,
			insertSQL,
			id,
			title,
			content,
			important != 0,
			createdAt,
			updatedAt,
		); err != nil {
			return count, fmt.Errorf("写入 PostgreSQL announcements.id=%d 失败: %w", id, err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return count, fmt.Errorf("遍历 SQLite 公告失败: %w", err)
	}
	return count, nil
}

func resetSequences(ctx context.Context, target *sql.Tx) error {
	for _, table := range []string{"query_logs", "announcements"} {
		query := fmt.Sprintf(`
			SELECT setval(
				pg_get_serial_sequence('%s', 'id'),
				COALESCE(MAX(id), 1),
				MAX(id) IS NOT NULL
			) FROM %s
		`, table, table)
		var sequenceValue int64
		if err := target.QueryRowContext(ctx, query).Scan(&sequenceValue); err != nil {
			return fmt.Errorf("重置 %s ID 序列失败: %w", table, err)
		}
	}
	return nil
}

func sqliteTableExists(ctx context.Context, source *sql.Tx, table string) (bool, error) {
	var exists bool
	if err := source.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?
		)
	`, table).Scan(&exists); err != nil {
		return false, fmt.Errorf("检查 SQLite 表 %s 失败: %w", table, err)
	}
	return exists, nil
}

func sqliteColumns(ctx context.Context, source *sql.Tx, table string) (map[string]bool, error) {
	if table != "query_logs" {
		return nil, fmt.Errorf("不支持读取表 %q 的字段", table)
	}
	rows, err := source.QueryContext(ctx, `PRAGMA table_info(query_logs)`)
	if err != nil {
		return nil, fmt.Errorf("读取 SQLite %s 表结构失败: %w", table, err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, fmt.Errorf("扫描 SQLite %s 表结构失败: %w", table, err)
		}
		columns[name] = true
	}
	return columns, rows.Err()
}

func columnOr(columns map[string]bool, column, fallback string) string {
	if columns[column] {
		return column
	}
	return fallback
}

func parseLegacyTime(value string, legacyLocation *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.ParseInLocation(layout, value, legacyLocation); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("不支持的时间格式 %q", value)
}
