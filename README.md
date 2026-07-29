# easy-qfnu-kjs

曲阜师范大学空教室查询系统。当前分支正在重构架构：后端使用 Go、Gin 和 PostgreSQL，旧前端已经移除，等待重新实现。

## 数据库

- 运行时数据库为 PostgreSQL 17。
- 数据表通过后端内置的版本化 SQL 迁移自动创建。
- 时间字段使用 `TIMESTAMPTZ`，数据层统一写入 UTC。
- 旧 SQLite 的查询日志和公告可通过一次性迁移工具导入，详见 [SQLite 到 PostgreSQL 迁移](docs/sqlite-to-postgresql.md)。

## 快速开始

复制并修改环境变量：

```bash
cp .env.example .env
```

至少需要配置：

- `QFNU_USERNAME`、`QFNU_PASSWORD`
- `OCR_URL`
- `ADMIN_USERNAME`、`ADMIN_PASSWORD`
- `JWT_SECRET`
- `POSTGRES_PASSWORD`

启动 PostgreSQL 和后端：

```bash
docker compose up -d --build
docker compose ps
```

后端默认只监听宿主机 `127.0.0.1:8080`，可通过 `BACKEND_PORT` 修改端口。

## 本地开发

先启动 PostgreSQL：

```bash
docker compose up -d postgres
```

确认 `.env` 中的 `DATABASE_URL` 指向本机 PostgreSQL，然后运行：

```bash
task install
task backend-dev
```

## 常用命令

```bash
task env-init
task install
task build
task up
task down
task logs
task ps
```

迁移旧数据：

```bash
task migrate-sqlite SQLITE_PATH=data/stats.db
```

## 远程部署

```bash
task deploy HOST=1.2.3.4 PORT=22 USER=root DIR=/srv/app
```

首次从 SQLite 切换到 PostgreSQL 时，应先按迁移文档停写、备份并导入数据，再启动新后端。
远程部署脚本会在首次切换时自动备份并迁移检测到的 `data/stats.db`。
