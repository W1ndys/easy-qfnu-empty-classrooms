package database

import "testing"

func TestMigrationVersion(t *testing.T) {
	version, err := migrationVersion("001_initial_schema.sql")
	if err != nil {
		t.Fatalf("解析迁移版本失败: %v", err)
	}
	if version != 1 {
		t.Fatalf("迁移版本错误: got=%d want=1", version)
	}
}

func TestMigrationVersionRejectsInvalidName(t *testing.T) {
	if _, err := migrationVersion("initial.sql"); err == nil {
		t.Fatal("无版本迁移文件名未被拒绝")
	}
}
