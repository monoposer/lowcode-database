# Lowcode Database (Postgres + HTTP JSON)

**[English](README.md) | [中文](README.zh.md)**

基于 Postgres 的低代码表服务：动态 schema、行 CRUD、虚拟列、保存的 Query。

隔离单位是 **租户**（`tenants.tenant_id`）：请求头 `X-Tenant-Id`。每个租户一条 **`data_dsn`**，读写共用。

## 功能概览

- **Table / Column / Index** — Admin API 管理 schema，DDL 作用在租户 data 库
- **Row CRUD** — 单行增删改、批量 upsert/delete、导入
- **Link** — 虚拟列（`type_id=link`）；关联 ID 在 `link_ref`，不进 `record.data`
- **lookup / formula / rollup** — 缓存在 `record.data`，由 `calc_queue` + 进程内 worker 刷新
- **Query** — 列投影 + filter + sort，经 `POST /v1/data/queries/{name}` 执行
- **ER 图** — 边来自 Link 列（`GET /v1/admin/schema/er`）
- **HTTP JSON** — `net/http`，无 gRPC；admin + data + calc **同一进程**
- **双库** — 共享 meta 库 + 每租户 `data_dsn`

## 环境要求

- Go 1.25+
- Postgres（本地推荐 `make docker-up` 启动 Postgres + Redis）

## 快速开始

```bash
cp .env.example .env
make docker-up         # postgres + redis（空库，不自动 apply SQL）
make migrate           # 执行 SQL 迁移
make run               # :8080  /v1/admin + /v1/data + calc
make playground-dev    # :5173  调试 UI（web/playground）
```

所有 API 请求需带 `X-Tenant-Id`。

启动后：

- **Control（Admin）**：`http://localhost:8080/v1/admin/*`
- **Data**：`http://localhost:8080/v1/data/*`
- **OpenAPI + Swagger UI**：`http://localhost:8080/swagger/`
- **Playground**：`http://localhost:5173`（`make playground-dev`）

### 配置说明

| 变量 | 说明 |
|------|------|
| `META_DATABASE_URL` | Meta 库（`tenants`、`lc_*`） |
| `DATA_DATABASE_URL` | 本地默认租户 data DSN（创建租户时作为 `data_dsn`） |
| `HTTP_ADDR` | 监听地址（默认 `:8080`） |
| `REDIS_URL` + `CACHE_ENABLED` | 可选元数据缓存 |
| `API_KEY_REQUIRED` | 为 true 时 `/v1/*` 需有效 `X-Api-Key` |
| `PG_STAT_STATEMENTS` | 为 true 时开放 `GET /v1/admin/pg-stat-statements` |

完整选项见 [`.env.example`](.env.example)。

### 多租户

每个租户是 `tenants` 一行，带 **`data_dsn`**。用 `POST /v1/admin/tenants` 创建（同时种子 **public** base 与默认 API key，明文 `key` 仅返回一次）；请求始终带 `X-Tenant-Id`。`cmd/migrate` 与 server 启动都不会自动写入租户。

## API 结构

| 前缀 | 进程 | 用途 |
|------|------|------|
| `/v1/admin/*` | `cmd/server` | Schema 与平台（Meta） |
| `/v1/data/*` | `cmd/server` | 行读写、执行保存的 Query |

OpenAPI：[`internal/api/openapi/openapi.yaml`](internal/api/openapi/openapi.yaml)

可选 API Key：`API_KEY_REQUIRED` + `X-Api-Key`。本服务没有 RBAC。

## Playground（调试 UI）

本仓库 [`web/playground`](web/playground)。

```bash
make run              # Admin + Data + calc :8080
make playground-dev   # UI :5173
```

侧边栏设置 **X-Tenant-Id**（默认 `default`）。需要时用 `VITE_DATA_API_BASE` 指向数据面。

## Link 与 lookup

Link 元数据在 `lc_columns`（`type_id=link`）。**关联 ID 只存在 `link_ref`**，不写入 `record.data`。

lookup / formula / rollup 结果缓存在 `record.data`（`{value, _cache_status}`），由 `calc_queue` worker 刷新。列表读缓存；详情 `GET /v1/data/tables/{id}/rows/{rowId}` 可现场计算。见 [docs/architecture/record-calc.md](docs/architecture/record-calc.md)。

### link（虚拟列）

无独立 PG 列。`config` 需要 `target_table_name`（或 `to_table_name`）。可选：

- **many** — `cardinality=many` 和/或对端 `link_column_id`
- **one** — `cardinality=one` 和/或 `target_column_id`

行 API 用 ID 写 Link（存 `link_ref`）。图展开 / `saveGraph` 见 [docs/roadmap.md](docs/roadmap.md)。

### lookup / formula / rollup

缓存在 `record.data`；单行写 API 只读。worker 经 `calc_queue` 刷新。

## 行 JSON 格式

创建、更新、列表响应使用**扁平行**：列名与 `id` 同级，标量为原生 JSON 类型。

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "amount": 99.5,
  "vendor_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

| 列类型 | 能否写入 | 说明 |
|--------|----------|------|
| 标量 / choice | 是 | 存在 `record.data` |
| link | 是（ID） | 写入 `link_ref` |
| lookup | 否 | 写底层 link ID |
| formula / rollup | 否 | 只读缓存 |

### 常用 Data API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/data/tables/{tableName}/rows` | 分页列表 |
| GET | `/v1/data/tables/{tableName}/rows/{rowId}` | 详情（可现场算 formula/lookup/rollup） |
| POST | `/v1/data/tables/{tableName}/rows` | 创建单行 |
| PATCH | `/v1/data/tables/{tableName}/rows/{rowId}` | 更新单行 |
| DELETE | `/v1/data/tables/{tableName}/rows/{rowId}` | 删除 |
| POST | `/v1/data/tables/{tableName}/rows:query` | DSL 过滤查询 |
| POST | `/v1/data/tables/{tableName}/rows:bulkUpsert` | 批量 upsert（单表、事务内） |
| POST | `/v1/data/tables/{tableName}/rows:bulkDelete` | 按 id 批量删除 |
| POST | `/v1/data/queries/{name}` | 执行保存的查询 |

## 常用命令

```bash
make migrate          # meta + data SQL 迁移
make run              # admin + data + calc
make playground-dev   # 调试 UI
make test             # 单元测试（集成测试需 TEST_META_DATABASE_URL）
make docker-build     # 构建镜像（见 deploy/）
make docker-up        # 仅 postgres + redis（空库，不自动 apply SQL）
make docker-migrate   # 通过 compose migrate 服务执行迁移
make docker-up-stack  # 含应用的完整栈（deploy/docker-compose.yml）
```

## 部署与发布

- Docker：[`deploy/Dockerfile`](deploy/Dockerfile)、[`deploy/docker-compose.yml`](deploy/docker-compose.yml)
- 发布：打 tag `v*.*.*` 推送 Docker Hub（[`deploy/RELEASE.md`](deploy/RELEASE.md)）
- 版本文件：[`VERSION`](VERSION)

## 目录结构

| 路径 | 说明 |
|------|------|
| `cmd/server/` | 运行时：`/v1/admin/*` + `/v1/data/*` + calc |
| `cmd/migrate/` | Meta schema 迁移 CLI |
| `web/playground/` | 调试 UI |
| `internal/api/` | 路由与 handler |
| `internal/service/{schema,catalog,data,platform,shared}/` | 手写 JSON 类型（无 proto） |
| `internal/service/` | 业务逻辑 |
| `migrations/` | SQL 迁移文件 |

## 延伸阅读

- [docs/](docs/README.md) — 技术架构、模块索引
- [AGENTS.md](AGENTS.md) — 开发者 / Agent 速查
