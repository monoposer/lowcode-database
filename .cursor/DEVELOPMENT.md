# 本地开发（Cursor）

## 环境变量

当前代码使用 **双库模型**（非 README 里旧的 `TENANT_MODE`）：

```bash
# Meta：所有 lc_* 元数据 + lc_tenants
META_DATABASE_URL=postgresql://postgres:postgres@localhost:5432/lowcode_meta

# 默认租户的数据库（物理 lc_t_* 表 + PG ENUM）
DEFAULT_TENANT_DATA_DSN=postgresql://postgres:postgres@localhost:5432/lowcode_data
DEFAULT_TENANT_ID=default

HTTP_ADDR=:8080
MAX_ROW=100

# 可选：Redis 元数据缓存 + metrics 后端
# REDIS_URL=redis://localhost:6379/0
# CACHE_ENABLED=true
# CACHE_TTL_SECONDS=300
# METRICS_BACKEND=prometheus   # noop | redis | prometheus
# METRICS_WINDOW_SIZE=100
# LOG_LEVEL=info
# SLOW_QUERY_THRESHOLD_MS=500

# 可选：API 创建租户时自动 CREATE DATABASE
# DATA_ADMIN_DATABASE_URL=postgresql://postgres:postgres@localhost:5432/postgres
# DATA_DSN_TEMPLATE=postgresql://postgres:postgres@localhost:5432/%s
```

复制：`cp .env.example .env` 并按上表修改。

## 启动与调试

```bash
make docker-up      # postgis/postgis:16-3.5 + init（含 lowcode_data 上 CREATE EXTENSION postgis）
make migrate        # 或手动：go run ./cmd/migrate -target meta && ... data
make run            # HTTP 服务（不跑 migration）
make test
make test-integration
```

Playground UI 见独立仓库 **lowcode-database-playground**。

集成测试：

```bash
export TEST_META_DATABASE_URL='postgresql://postgres:postgres@localhost:5432/lowcode_meta'
export TEST_DATA_DATABASE_URL='postgresql://postgres:postgres@localhost:5432/lowcode_data'
make test-integration
```

HTTP 调试示例：

```bash
curl -H 'X-Tenant-Id: default' http://localhost:8080/v1/tables
curl -H 'X-Tenant-Id: default' http://localhost:8080/v1/schema/er
```

## 双库职责

```
HTTP /v1/*
    → internal/service/LowcodeService
    → MetaPool (META_DATABASE_URL)     — 表/列/视图/枚举注册
    → DataPool (lc_tenants.data_dsn)   — lc_t_* 行数据、ENUM、INDEX
```

- 启动时 `db.NewTenantManager` **不** 执行 migration
- Schema 来源：`docker/postgres/migrations/` + `cmd/migrate` 或 Docker init

## Migration

SQL 文件：`docker/postgres/migrations/meta/`（data 目录无 `.up.sql`，见 [data/README.md](../docker/postgres/migrations/data/README.md)）

| 命令 | 说明 |
|------|------|
| `make docker-up` | 首次 volume 空时 init 脚本自动 apply meta migrations |
| `make migrate` | `cmd/migrate` 对 meta apply；data 无 pending migration |
| `go run ./cmd/migrate -target meta -database-url '...'` | 单库迁移 |

**Data 库**：无 `.up.sql` migration。UUID 主键用 PG 内置 `gen_random_uuid()`。PostGIS：Docker 用 `postgis/postgis` 镜像，init 在 `lowcode_data` 启用；自建/云库见 [data/README.md](../docker/postgres/migrations/data/README.md)。

版本记录在 `lc_schema_migrations`。新增 migration：追加 `000012_xxx.up.sql`，再 `make migrate-meta`。

**勿**在 `internal/service` 或 `cmd/server` 内嵌 SQL schema。

## API 概览

前缀 `/v1/`，JSON camelCase，租户头 **`X-Tenant-Id`** 必填。

| 域 | 主要路由 |
|----|----------|
| Tenant | `POST /v1/tenants` |
| Table | `GET/POST /v1/tables`，`GET .../schema`，`POST ...:rename`，`DELETE .../{name}` |
| Column | `GET/POST /v1/columns?table_id=`，`PATCH/DELETE /v1/columns/{id}` |
| Row | `GET/POST .../rows`，`POST .../rows:query`，`PATCH/DELETE .../rows/{id}`，bulk/import |
| Index | `GET/POST /v1/indexes?table_id=`，`GET/DELETE /v1/indexes/{pgIndexName}` |
| Choice (ENUM) | `GET/POST /v1/choices`，`GET/PATCH/DELETE /v1/choices/{id}` |
| Relation | `GET/POST /v1/relations`，`DELETE /v1/relations/{id}` |
| DataSource | `GET/POST /v1/data-sources`，`POST /v1/data-sources/{id}:query`（列表/视图定义 + 查询） |
| ER | `GET /v1/schema/er` |
| Event sink | `GET/POST /v1/admin/event-sinks`，`PATCH/DELETE /v1/admin/event-sinks/{id}`（原 `/v1/webhooks` 已 404） |

## 领域约定

### Choice（PostgreSQL ENUM）

- 创建：`CREATE TYPE {schema}.{name} AS ENUM ('a','b')`（仅 data DB，无 meta 表；类型名 = logical name）
- 列表/读取：查 `pg_type` / `pg_enum`（`public` 下合法标识符；兼容旧 `lc_e_{tenant}_*`）
- 列类型 `enum` + `config.choice_name`（逻辑名）或 `choice_id`（同逻辑名 / 完整 pg 类型名）
- 更新枚举：`replaceValues=true` 时全量替换（删值会重建 ENUM 类型；若行数据仍引用被删 label 会报错）；否则 `ALTER TYPE ... ADD VALUE IF NOT EXISTS` 追加
- `Values` API 字段从 `pg_enum` 读取

### Index

- **Source of truth**：PostgreSQL catalog（`internal/service/pg_catalog.go`）
- 创建：`CREATE [UNIQUE] INDEX IF NOT EXISTS idx_{table}_{name} ON ...`
- 列表/Schema：`listPGIndexes` + `pgIndexesToAPI`
- `Index.Id` = PG 索引名（如 `idx_vendor_score`）

### 虚拟列

| kind | 说明 |
|------|------|
| relationship | 一对多 / 多对一；config 中 link_column_id 与 target_column_id 互斥 |
| lookup | 基于 cardinality one 的 relationship LEFT JOIN |
| formula | Excel 表达式（`config.expression`）；`{{column_name}}` → [pg-formula](https://github.com/SolaTyolo/pg-formula) 编译为 PostgreSQL 标量 SQL |
| rollup | 对关联表聚合子查询 |

### 查询

- DSL：`internal/dsl`（metadata 兼容 JSON shape）
- 合并：DataSource 的 filter + sort + columnIds；`POST .../rows:query` 或 DataSource `:query`

## 改代码时的模式

1. **API 类型**：`internal/apiv1/`（必要时 `types.go` + `extended_types.go`）
2. **业务**：`internal/service/<domain>_service.go`，共享逻辑放 `service_helpers.go`、`pg_catalog.go`
3. **路由**：`internal/api/handler.go` 的 `dispatch` / `handleTablesSubtree`
4. **测试**：单元测试同包；集成测试 `internal/service/integration_test.go` + `internal/testutil`

保持小 diff，与现有 `*_service.go` 拆分风格一致。

## 常见坑

- 无 `X-Tenant-Id` → 400
- `loadColumns` 不含虚拟列；查索引用 `loadTablePhysical`
- ENUM 列 NOT NULL 时插入行必须带值
- 删 ENUM 类型前需先删引用该类型的列
- README 中 proto / TENANT_MODE / GRPC 描述已过时，以本文与 `AGENTS.md` 为准
