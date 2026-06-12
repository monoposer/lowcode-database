# Lowcode Database (Postgres + HTTP JSON)

**[English](README.md) | [中文](README.zh.md)**

基于 Postgres 的低代码表格服务：动态 schema、行 CRUD、虚拟列、DataSource 查询与事件投递。

## 功能概览

- **Table / Column / Index / Choice（PG ENUM）** — 通过 Admin API 管理 schema，DDL 作用于租户 data 库
- **Row CRUD** — 单行增删改、批量 upsert/delete、导入、嵌套 `saveGraph`
- **虚拟列** — `relationship`、`lookup`、`formula`、`rollup`（无 PG 物理列）
- **relation_fk** — 物理外键列，可选 `FOREIGN KEY` 约束
- **DataSource** — 列投影 + filter + sort，经 Data API 查询
- **Relation 与 ER 图** — `lc_relations` + schema ER API
- **Relationship 展开** — ListRows 时 `expand_column_ids` / `expand_paths`
- **事件投递** — 行或 schema 变更后异步推送到 sink（HTTP、Redis、Kafka、RabbitMQ 等）；见 [pkg/eventschema](pkg/eventschema/)
- **HTTP JSON API** — `net/http`，无 gRPC；分为 **Admin** 与 **Data** 两套路由
- **双库架构** — 共享 meta 库 + 每租户独立 data 库

## 环境要求

- Go 1.22+
- Postgres（本地推荐 `make docker-up` 启动 Postgres + Redis）

## 快速开始

```bash
cp .env.example .env
make docker-up    # postgres + redis
make migrate      # 执行 SQL 迁移
make run          # http://localhost:8080
```

所有 API 请求需带请求头 `X-Tenant-Id: default`（或你的租户 id）。

启动后：

- **HTTP API**：`http://localhost:8080/`
- **OpenAPI + Swagger UI**：`http://localhost:8080/swagger/`
- **版本信息**：`GET /` 或 `./server -version`

### 配置说明

| 变量 | 说明 |
|------|------|
| `META_DATABASE_URL` | Meta 库（`lc_*` 表、租户信息） |
| `DEFAULT_TENANT_DATA_DSN` | 默认租户 data 库（`lc_t_*` 物理表） |
| `DEFAULT_TENANT_ID` | 启动时注册的租户 id（默认 `default`） |
| `REDIS_URL` + `CACHE_ENABLED` | 可选元数据缓存 |
| `API_KEY_REQUIRED` | 为 true 时 `/v1/*` 需有效 `X-Api-Key` |
| `AUTHZ_DRIVER` | `file` 或 `http` 做 RBAC（示例见 `config/authz.example.json`） |

完整选项见 [`.env.example`](.env.example)。

### 多租户

每个租户在 meta（`lc_tenants`）中记录独立 data DSN。通过 `POST /v1/admin/tenants` 创建租户；请求时始终携带 `X-Tenant-Id`。

## API 结构

旧路径 `/v1/tables`、`/v1/webhooks` 等已返回 **404**。请使用：

| 前缀 | 用途 | 示例 |
|------|------|------|
| `/v1/admin/*` | Schema 与平台管理 | `GET /v1/admin/tables`、`POST /v1/admin/columns`、`GET /v1/admin/schema/er` |
| `/v1/data/*` | 行读写与查询 | `GET /v1/data/tables/{tableId}/rows`、`POST /v1/data/tables/{tableId}/rows:saveGraph` |

OpenAPI：[`internal/api/openapi/openapi.yaml`](internal/api/openapi/openapi.yaml)

## 事件投递

通过 Admin API 配置 **event sink**。行数据或 schema 变更后，服务异步向 sink 的 delivery URL 推送 JSON envelope。

Envelope 结构（详见 [pkg/eventschema](pkg/eventschema/README.zh.md)）：

```json
{
  "type": "records.after.insert",
  "tenantId": "default",
  "tableId": "orders",
  "occurredAt": "2026-06-12T10:00:00Z",
  "data": { "row": { "id": "...", "amount": { "numberValue": 99.5 } } }
}
```

**Admin 接口：**

- `GET /v1/admin/event-sinks` — 列表
- `POST /v1/admin/event-sinks` — 创建（`name`、`targetUrl`、`eventTypes`、`tableFilter`、`enabled` 等）
- `PATCH /v1/admin/event-sinks/{id}` — 更新
- `DELETE /v1/admin/event-sinks/{id}` — 删除
- `GET /v1/admin/events/schemas` — 全部事件类型的 JSON Schema

**投递：** 仅支持 `http://` / `https://` 的 `targetUrl`（HTTP POST）。Kafka、Redis、RabbitMQ、SQS、SNS 等请部署 **HTTP 适配器**，将 `targetUrl` 指向该服务。鉴权：`headers` + 可选 `secret`（HMAC `X-Lowcode-Signature`）。详见 [docs/事件投递.md](docs/事件投递.md)。

**订阅规则：**

- `events` 为空 → 订阅全部行级 `records.*` 事件
- `eventTypes` 非空 → 仅投递列表中的 `type`
- `tableFilter` 非空 → 仅匹配该逻辑表名

## Playground（调试 UI）

独立仓库：[lowcode-database-playground](https://github.com/solat/lowcode-database-playground)

```bash
make run   # API :8080

git clone https://github.com/solat/lowcode-database-playground.git
cd lowcode-database-playground
cp .env.example .env && npm install && npm run dev   # :5173
```

侧边栏设置 **X-Tenant-Id**（默认 `default`）。

## Relationship、Lookup 与展开查询

### relationship（虚拟列）

无 PG 存储列。`config` 中 **`link_column_id` 与 `target_column_id` 只能二选一**：

- **many（一对多）** — 子表 FK 列指向当前行（`link_column_id`）
- **one（多对一 / 一对一）** — 当前表 FK 列指向目标行（`target_column_id`）

**ListRows — `expand_column_ids`：**

- `many` → `{ "rows": [ { "id", "cells": { … } }, … ] }`
- `one` → `{ "id", "cells": { … } }` 或 `null`

示例：`GET /v1/data/tables/{tableId}/rows?expand_column_ids=col-uuid-1`

### expand_paths（点分路径）

逗号分隔的多条路径，每段为列 id，至少两段，最大深度 5。示例：

`GET /v1/data/tables/{tableId}/rows?expand_paths=relToOrder.orderName`

结果写入行内，**键为完整路径字符串**。

### lookup（虚拟列）

在 **one** 关系的 relationship 上投影关联表某一物理列。配置：`relation_column_id`、`target_column_id`。ListRows 通过 JOIN 计算；单行写入 API 只读。

### relation_fk（物理外键列）

data 库真实 PG 列，类型与目标列一致。配置：`target_table_id`、可选 `target_column_id`、`add_fk`。

### formula / rollup

只读计算列（`expression` / 对 relationship 子表聚合）。

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
| 标量 / choice / relation_fk | 是 | 直接写 PG 列 |
| relationship | 否 | 通过 link 列或 target FK 列关联 |
| lookup | 否（单行 API） | 写底层 FK；或使用 `saveGraph` |
| formula / rollup | 否 | 只读 |

### 常用 Data API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/data/tables/{tableId}/rows` | 分页列表，支持 expand |
| POST | `/v1/data/tables/{tableId}/rows` | 创建单行 |
| PATCH | `/v1/data/tables/{tableId}/rows/{rowId}` | 更新单行 |
| DELETE | `/v1/data/tables/{tableId}/rows/{rowId}` | 删除 |
| POST | `/v1/data/tables/{tableId}/rows:query` | DSL 过滤查询 |
| POST | `/v1/data/tables/{tableId}/rows:bulkUpsert` | 批量 upsert（单表、事务内） |
| POST | `/v1/data/tables/{tableId}/rows:saveGraph` | 嵌套保存主行 + relationship |
| POST | `/v1/data/tables/{tableId}/rows:bulkDelete` | 按 id 批量删除 |

## 嵌套保存 `rows:saveGraph`

`POST /v1/data/tables/{tableId}/rows:saveGraph` 在单事务内保存主行及 relationship 嵌套数据：

| cardinality | payload | 行为 |
|-------------|---------|------|
| **one** | object | 在关联表 create/update，写入本表 FK |
| **many** | array | upsert 子行，填充 link 列 |

整单替换子行时加 `"_sync": { "items": "replace" }`。

**请求示例：**

```json
{
  "order_remark": "rush",
  "supply": { "code": "SUP-001" },
  "items": [
    { "qty": 2, "goods": { "name": "Apple" } },
    { "qty": 1, "goods_name": "Banana" }
  ]
}
```

- **one 嵌套**（如 `supply`）：在 supply 表创建/更新行，并写入 `supply_id`。
- **many 数组**（如 `items`）：upsert 子行；默认 merge。整单替换加 `_sync`。
- **lookup 列名**（如 `goods_name`）：按 schema 查已有行并写 FK。
- **formula / rollup**：忽略。

## 常用命令

```bash
make migrate          # meta + data SQL 迁移
make run              # 启动 HTTP 服务
make test             # 单元测试（集成测试需 TEST_META_DATABASE_URL）
make docker-build     # 构建镜像（见 deploy/）
make docker-up        # 仅 postgres + redis
make docker-up-stack  # 含应用的完整栈（deploy/docker-compose.yml）
```

## 部署与发布

- Docker：[`deploy/Dockerfile`](deploy/Dockerfile)、[`deploy/docker-compose.yml`](deploy/docker-compose.yml)
- 发布：打 tag `v*.*.*` 推送 Docker Hub（[`deploy/RELEASE.md`](deploy/RELEASE.md)）
- 版本文件：[`VERSION`](VERSION)

## 目录结构

| 路径 | 说明 |
|------|------|
| `cmd/server/` | HTTP 入口 |
| `cmd/migrate/` | Schema 迁移 CLI |
| `internal/api/` | 路由与 handler |
| `internal/apiv1/` | 手写 JSON 类型 |
| `internal/service/` | 业务逻辑 |
| `internal/event/` | 事件总线与投递 |
| `pkg/eventschema/` | 对外事件 JSON Schema |
| `docker/postgres/migrations/` | SQL 迁移文件 |

## 延伸阅读

- [docs/](docs/README.md) — 技术架构、模块索引、架构分析（中文）
- [AGENTS.md](AGENTS.md) — 开发者 / Agent 速查
