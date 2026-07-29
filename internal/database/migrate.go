package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

const migrationLockName = "easy-qfnu-kjs-schema-migrations"

// Migrate 按文件名版本顺序执行尚未应用的 PostgreSQL 迁移。
func Migrate(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("获取迁移连接失败: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock(hashtext($1))`, migrationLockName); err != nil {
		return fmt.Errorf("获取数据库迁移锁失败: %w", err)
	}
	defer conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(hashtext($1))`, migrationLockName)

	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("创建迁移记录表失败: %w", err)
	}

	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("读取迁移文件失败: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, err := migrationVersion(entry.Name())
		if err != nil {
			return err
		}

		var applied bool
		if err := conn.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`,
			version,
		).Scan(&applied); err != nil {
			return fmt.Errorf("检查迁移 %s 状态失败: %w", entry.Name(), err)
		}
		if applied {
			continue
		}

		content, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("读取迁移 %s 失败: %w", entry.Name(), err)
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("开始迁移 %s 失败: %w", entry.Name(), err)
		}
		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("执行迁移 %s 失败: %w", entry.Name(), err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
			version, entry.Name(),
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("记录迁移 %s 失败: %w", entry.Name(), err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("提交迁移 %s 失败: %w", entry.Name(), err)
		}
	}
	return nil
}

func migrationVersion(name string) (int64, error) {
	prefix, _, ok := strings.Cut(name, "_")
	if !ok {
		return 0, fmt.Errorf("迁移文件名 %q 缺少版本分隔符", name)
	}
	version, err := strconv.ParseInt(prefix, 10, 64)
	if err != nil || version <= 0 {
		return 0, fmt.Errorf("迁移文件名 %q 的版本无效", name)
	}
	return version, nil
}
