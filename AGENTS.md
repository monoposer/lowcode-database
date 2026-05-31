# lowcode-database — Cursor / Agent 开发说明

基于 Postgres 的低代码表服务：HTTP JSON API（`/v1/*`），动态表/列/行/视图/枚举，双库架构（meta + data）。

## 关联仓库

| 仓库 | 关系 |
|------|------|
| treelab-metadata | Entity/View/Choice 元数据模式参考 |
| treelab-scm-service | 行级查询、View 数据源、DSL 过滤参考 |
| [lowcode-database-playground](https://github.com/solat/lowcode-database-playground) | Vite + AG Grid 调试 UI（独立仓库） |

## 本地启动

1. `cp .env.example .env`
2. **Docker（推荐）**：`make docker-up` — 首次启动自动建库 + apply SQL
3. 或自备 Postgres：`make migrate`（`cmd/migrate`）
4. `make run` → http://localhost:8080
5. API 请求带 `X-Tenant-Id: default`

## 目录

| 路径 | 说明 |
|------|------|
| `cmd/server/` | HTTP 入口 |
| `cmd/migrate/` | Schema 迁移 CLI |
| `internal/api/` | 路由与 handler |
| `internal/apiv1/` | JSON 请求/响应类型（手写，无 proto） |
| `internal/service/` | 业务逻辑（按域拆分 `*_service.go`） |
| `internal/dsl/`、`internal/query/` | 过滤 DSL → SQL |
| `internal/cache/` | Redis 元数据缓存（data source / view / column spec） |
| `internal/metrics/` | DataSource 查询 metrics（最近 N 次平均耗时） |
| `internal/logger/` | JSON 结构化日志 |
| `docker/postgres/migrations/` | Meta/Data SQL 迁移文件 |
| `internal/db/` | 双库 TenantManager |

调试 UI 见独立仓库 **lowcode-database-playground**。

## 架构要点

- **Meta DB**：`lc_tables`、`lc_columns`、`lc_choices`（ENUM 注册）、`lc_relations`、`lc_data_sources`、`lc_tenants` 等
- **Data DB**：物理表 `lc_t_*`、PG ENUM 类型、索引（以 PG catalog 为准）
- **Choice**：data DB 的 PG ENUM（类型名与 logical name 相同），catalog 为唯一来源
- **Index**：读写直接对接 PostgreSQL（`pg_index` / `pg_class`），不依赖 `lc_indexes` 镜像

## 性能与可观测性

- **Redis 缓存**（`REDIS_URL` + `CACHE_ENABLED`）：缓存 data source / view / column 元数据，写操作自动失效
- **Metrics**（`METRICS_BACKEND=redis|prometheus`）：记录每个 data source 最近 100 次（可配）查询平均耗时
  - Prometheus：`GET /metrics` 暴露 `lowcode_datasource_query_avg_seconds` 等
  - Redis：List 存储 rolling window
- **日志**：JSON stdout；`SLOW_QUERY_THRESHOLD_MS` 触发 datasource / SQL 慢查询 warn

```bash
make docker-up   # postgres + redis
export REDIS_URL=redis://localhost:6379/0
export METRICS_BACKEND=prometheus
make run
```

## 勿混淆

- 无 gRPC / protobuf；勿添加 `make proto`
- `Table.Id` 对外为逻辑 **name**，不是 UUID
- 虚拟列（`formula` / `relationship` / `lookup` / `rollup`）无 PG 物理列
- 改 schema：**编辑 `docker/postgres/migrations/`**，执行 `make migrate` 或 `docker compose up` 首次初始化
- 业务服务 **不** 自动跑 migration

详见 [.cursor/DEVELOPMENT.md](.cursor/DEVELOPMENT.md)
