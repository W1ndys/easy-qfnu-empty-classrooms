package service

import (
	"crypto/subtle"
	"database/sql"
	"fmt"
	"strings"
	"sync"

	"github.com/W1ndys/easy-qfnu-kjs/internal/model"
	"github.com/W1ndys/easy-qfnu-kjs/pkg/logger"
)

// OpenAPIConfigService 管理开放查询接口的启用状态和访问密钥。
type OpenAPIConfigService struct {
	db *sql.DB
	mu sync.RWMutex
}

func NewOpenAPIConfigService(db *sql.DB) (*OpenAPIConfigService, error) {
	if err := migrateOpenAPIConfigSchema(db); err != nil {
		return nil, fmt.Errorf("开放接口配置表迁移失败: %w", err)
	}
	logger.Info("开放接口配置服务已初始化")
	return &OpenAPIConfigService{db: db}, nil
}

func migrateOpenAPIConfigSchema(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开始开放接口配置迁移失败: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS open_api_config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			enabled INTEGER NOT NULL DEFAULT 0,
			api_key TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		)
	`); err != nil {
		return fmt.Errorf("创建开放接口配置表失败: %w", err)
	}

	var legacyTableExists int
	if err := tx.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name = 'api_config'
	`).Scan(&legacyTableExists); err != nil {
		return fmt.Errorf("检查旧配置表失败: %w", err)
	}
	if legacyTableExists != 0 {
		if _, err := tx.Exec(`
			INSERT OR REPLACE INTO open_api_config (id, enabled, api_key, updated_at)
			SELECT 1, open_api_enabled, open_api_key, ? FROM api_config WHERE id = 1
		`, nowUTCForStorage()); err != nil {
			return fmt.Errorf("迁移旧开放接口配置失败: %w", err)
		}
		if _, err := tx.Exec(`DROP TABLE api_config`); err != nil {
			return fmt.Errorf("删除旧 AI 配置表失败: %w", err)
		}
	}

	if _, err := tx.Exec(`INSERT OR IGNORE INTO open_api_config (id) VALUES (1)`); err != nil {
		return fmt.Errorf("初始化开放接口配置失败: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交开放接口配置迁移失败: %w", err)
	}
	return nil
}

func (s *OpenAPIConfigService) Get() (*model.OpenAPIConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getLocked()
}

func (s *OpenAPIConfigService) getLocked() (*model.OpenAPIConfig, error) {
	var cfg model.OpenAPIConfig
	var enabled int
	var storedUpdatedAt string
	if err := s.db.QueryRow(`
		SELECT enabled, api_key, updated_at FROM open_api_config WHERE id = 1
	`).Scan(&enabled, &cfg.APIKey, &storedUpdatedAt); err != nil {
		return nil, fmt.Errorf("读取开放接口配置失败: %w", err)
	}

	updatedAt, err := utcTimestampForAPI(storedUpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("解析开放接口配置更新时间失败: %w", err)
	}
	cfg.Enabled = enabled != 0
	cfg.UpdatedAt = updatedAt
	return &cfg, nil
}

func (s *OpenAPIConfigService) Update(enabled bool, apiKey string) (*model.OpenAPIConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	enabledValue := 0
	if enabled {
		enabledValue = 1
	}
	if _, err := s.db.Exec(`
		UPDATE open_api_config SET enabled = ?, api_key = ?, updated_at = ? WHERE id = 1
	`, enabledValue, strings.TrimSpace(apiKey), nowUTCForStorage()); err != nil {
		return nil, fmt.Errorf("保存开放接口配置失败: %w", err)
	}
	return s.getLocked()
}

func (s *OpenAPIConfigService) ValidateKey(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	cfg, err := s.Get()
	if err != nil || !cfg.Enabled || cfg.APIKey == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(key), []byte(cfg.APIKey)) == 1
}
