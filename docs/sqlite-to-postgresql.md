# SQLite 到 PostgreSQL 数据迁移

迁移工具位于 `cmd/migrate-sqlite-to-postgres`，负责导入：

- `query_logs`：查询日志、结果数量、IP 与 User-Agent 哈希
- `announcements`：公告内容和时间

已删除的 AI、开放接口配置不会迁移。

## 安全策略

迁移遵循以下约束：

1. SQLite 使用只读事务，迁移过程不会修改旧数据库。
2. PostgreSQL 使用单个事务并以排他锁锁定目标表。
3. 默认要求目标 `query_logs` 和 `announcements` 都为空，避免 ID 冲突或覆盖线上数据。
4. 任意一行失败会回滚整个 PostgreSQL 事务。
5. 导入后校验两张表的源端和目标端行数，并重置 PostgreSQL Identity 序列。
6. 工具可重复执行；仅在明确传入 `-allow-non-empty` 时才会按主键执行幂等更新。

## 推荐切换流程

### 1. 停止旧后端并备份

迁移期间不得继续向 SQLite 写入。停止旧后端后，保留数据库及 WAL 文件：

```bash
cp -a data/stats.db data/stats.db.backup
test ! -f data/stats.db-wal || cp -a data/stats.db-wal data/stats.db-wal.backup
test ! -f data/stats.db-shm || cp -a data/stats.db-shm data/stats.db-shm.backup
```

如果系统安装了 `sqlite3`，建议在备份前执行一次 checkpoint：

```bash
sqlite3 data/stats.db 'PRAGMA wal_checkpoint(TRUNCATE);'
```

### 2. 启动空 PostgreSQL

配置 `.env` 中的 PostgreSQL 用户和密码，然后仅启动数据库：

```bash
docker compose up -d postgres
docker compose ps postgres
```

### 3. 执行迁移

从宿主机运行：

```bash
task migrate-sqlite SQLITE_PATH=data/stats.db
```

也可以使用后端镜像中内置的迁移工具：

```bash
docker compose run --rm \
  -v "$PWD/data:/legacy:ro" \
  backend \
  /app/migrate-sqlite-to-postgres \
  -sqlite /legacy/stats.db
```

工具默认从 `DATABASE_URL` 读取目标地址。

### 4. 确认计数并启动后端

工具成功时会输出两张表的迁移行数。确认无误后启动后端：

```bash
docker compose up -d backend
docker compose logs --tail=200 backend
```

旧 SQLite 备份应至少保留到新系统经过完整验证之后。

### 远程部署自动迁移

`task deploy` 在远端检测到 `data/stats.db` 且不存在迁移标记时，会：

1. 先启动 PostgreSQL，暂不启动后端；
2. 将 SQLite、WAL 和 SHM 文件复制为 `.pre-postgresql` 备份；
3. 在事务中运行迁移工具；
4. 成功后创建 `data/.sqlite-to-postgresql-migrated` 标记；
5. 最后启动后端。

迁移失败时部署立即中止，不会启动新后端，也不会写入迁移标记。若旧公告使用中国时区，应在远端 `.env` 中提前设置：

```dotenv
SQLITE_QUERY_LOG_TIMEZONE=UTC
SQLITE_ANNOUNCEMENT_TIMEZONE=Asia/Shanghai
```

## 旧时间的时区

带 `Z` 或显式偏移的时间会直接转换为 UTC。没有时区的旧值需要告诉迁移工具其原始时区：

```bash
go run ./cmd/migrate-sqlite-to-postgres \
  -sqlite data/stats.db \
  -query-log-timezone UTC \
  -announcement-timezone Asia/Shanghai
```

- 历史 `query_logs.queried_at` 通常由 SQLite `datetime('now')` 产生，应使用 `UTC`。
- 早期公告曾使用 `datetime('now', 'localtime')`。如果旧容器设置过中国时区，应使用 `Asia/Shanghai`；Docker 默认 UTC 时仍使用 `UTC`。

## 向非空 PostgreSQL 导入

推荐始终使用空目标库。只有确认主键代表同一条业务数据时才可执行：

```bash
go run ./cmd/migrate-sqlite-to-postgres \
  -sqlite data/stats.db \
  -allow-non-empty
```

该模式会按 `id` 更新冲突行，不会删除 PostgreSQL 中额外存在的数据，因此总行数不再进行相等校验。

## 回滚

迁移工具不会修改 SQLite。若切换验证失败：

1. 停止新后端。
2. 保留 PostgreSQL volume 以便排查，不要执行 `docker compose down -v`。
3. 恢复旧版本程序和 SQLite 备份。
4. 修复问题后重新创建空 PostgreSQL 数据库并再次迁移。
