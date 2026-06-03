# Lowcode Database (Postgres + HTTP JSON)

一个基于 Postgres 的简易 low-code / nocode 表格服务，支持：

- **Type**：列类型定义
- **Table**：动态创建/删除逻辑表
- **Column**：按列增删改（底层 ALTER TABLE）
- **Row/Cell**：创建/更新/删除单行；`bulkUpsert` / `bulkDelete` 批量写
- **Index**：按列创建/删除索引
- **Relationship / Lookup / Formula / Rollup**：虚拟列（只读投影或计算；写入见下文）
- **relation_fk**：物理外键列，PG 类型与目标列一致，可选 `add_fk` 约束
- **Choice（PG ENUM）**：自定义枚举列
- **DataSource**：表视图（列投影 + filter + sort），`QueryDataSource` 查询
- **Relation 注册表**：`lc_relations` + ER 图 API
- **Relationship 展开**：ListRows 时 `expand_column_ids` / `expand_paths` 带出关联数据
- **Webhook（类 NocoDB）**：行级变更后向配置的 URL 发送 HTTP POST（JSON），可订阅 `records.after.insert` 等事件；见下文
- **HTTP JSON API**：`net/http` + `ServeMux`，路径 `/v1/*`（无 gRPC）
- 支持 **单库模式** 与 **多租户（数据库级隔离）模式**

## 环境要求

- Go 1.22+
- 一个可访问的 Postgres 实例（本地推荐 `make docker-up` 启动 Postgres + Redis）

## 运行服务

### 单例模式（默认，不开放多租户）

使用固定数据库（例如 `tables`）：

```bash
export TENANT_MODE=single
export SINGLE_DATABASE_URL='postgresql://postgres:postgres@0.0.0.0:5432/tables'
# 或者使用 DATABASE_URL（当 SINGLE_DATABASE_URL 未设置时）
# export DATABASE_URL='postgresql://postgres:postgres@0.0.0.0:5432/tables'

make run
```

服务启动后：

- **HTTP**（JSON API）：`http://localhost:8080/`
- API 前缀：`/v1/`（例如 `GET /v1/tables`）
- **API 文档（OpenAPI 3 + Swagger UI）**：`http://localhost:8080/swagger/`
- **调试 UI（Playground）**：见下文「Playground」

### 多租户模式（数据库级隔离）

每个租户使用一个独立的 Postgres 数据库，例如连接串为：

`postgresql://postgres:postgres@0.0.0.0:5432/<tenant_id>`

配置环境变量：

```bash
export TENANT_MODE=multi
export TENANT_DSN_TEMPLATE='postgresql://postgres:postgres@0.0.0.0:5432/%s'

make run
```

访问时在 HTTP 头中带上租户 ID：

- HTTP 头：`X-Tenant-Id: tenant_a`
- 将自动连接到：`postgresql://postgres:postgres@0.0.0.0:5432/tenant_a`

## Webhook（NocoDB 风格）

通过 API 配置 Webhook 后，系统在行数据变更后会 **异步** `POST` 到 `targetUrl`，请求体为 JSON：

```json
{
  "type": "records.after.insert",
  "tableId": "<逻辑表名>",
  "data": {
    "row": {
      "id": "...",
      "amount": 99.5,
      "name": "Acme"
    }
  }
}
```

支持的事件类型（`type`）包括：

- `records.after.insert` / `records.after.update` / `records.after.delete`
- `records.after.bulkUpsert` / `records.after.bulkDelete` / `records.after.bulkImport`

**订阅规则**：

- `events` 为空数组：订阅上述全部行级事件。
- `events` 非空：仅当 `type` 在列表中时才投递。
- `tableFilter` 非空时：仅当与当前逻辑表名一致时才投递；空字符串表示所有表。

若配置了 `secret`，请求会带 `X-Lowcode-Signature` 头，值为 **HMAC-SHA256(secret, body)** 的十六进制（与常见 webhook 验签方式类似）。

**HTTP 管理接口**：

- `GET /v1/webhooks` — 列表
- `POST /v1/webhooks` — 创建（body 含 `name`, `targetUrl`, `tableFilter`, `events`, `headers`, `enabled`, `secret` 等，camelCase JSON）
- `PATCH /v1/webhooks/{id}` — 更新
- `DELETE /v1/webhooks/{id}` — 删除

## Playground（调试 UI）

Playground 已独立为单独仓库：[lowcode-database-playground](https://github.com/solat/lowcode-database-playground)

```bash
# 先启动 API
make run              # http://localhost:8080

# 另开终端，克隆并启动 Playground
git clone https://github.com/solat/lowcode-database-playground.git
cd lowcode-database-playground
cp .env.example .env && npm install && npm run dev   # http://localhost:5173
```

侧边栏设置 **X-Tenant-Id**（默认 `default`）。

## Relationship、Lookup 与展开查询

### relationship（虚拟列）

列类型 **relationship**（虚拟列，无实际 PG 存储列）。`config` 中 **`link_column_id` 与 `target_column_id` 只能二选一**（写入时会规范化并写入 `cardinality`）：

- **一对多（cardinality `many`）**
  - `target_table_id`：子表 id（逻辑表名或 `lc_tables.name`）
  - `link_column_id`：子表中「存当前行 id」的外键列 id  
  - 查询：`子表.link 列 = 当前行 id`

- **多对一 / 一对一（cardinality `one`）**
  - `target_table_id`：目标表 id
  - `target_column_id`：当前表中存目标行 id 的列 id  
  - 查询：用当前行该列的值作为目标表主键 `id` 查一行

`cardinality` 在规范化后会自动设为 `one` 或 `many`；若误同时填写 `link_column_id` 与 `target_column_id`，创建/更新列会失败。

**ListRows — `expand_column_ids`**（relationship 列 id 列表，可多值 query 参数）：

- `cardinality` 为 **many**：该 relationship 列的值为 `{ "rows": [ { "id", "cells": { "列名": 值 } }, ... ] }`。
- `cardinality` 为 **one**：该列值为 `{ "id", "cells": { "列名": 值 } }`；无关联行时为 JSON `null`。

HTTP 示例：`GET /v1/tables/{table_id}/rows?expand_column_ids=col-uuid-1&expand_column_ids=col-uuid-2`

### expand_paths（点分路径）

**ListRows** 支持 `expand_paths`（或 `expandPaths`）：逗号分隔多条路径，每条为 **点分隔的列 id**，至少两段。第一段为当前表上的 relationship 列 id，最后一段为叶子列 id；中间段为下一层表上的 relationship 列 id。

- 解析失败（路径过短、深度超过 5、未知列等）会使整次 `ListRows` 返回错误。
- 对 **many** 关系，每一跳最多展开 **100** 条子行。

示例：`GET /v1/tables/{table_id}/rows?expand_paths=relToOrder.orderName`

结果写入行内对应键，**键为完整路径字符串**（如 `relToOrder.orderName`）。

### lookup（虚拟列）

列类型 **lookup**（虚拟列），用于在 **多对一 / 一对一** 的 relationship 上投影关联表某一物理列。`config`：

- `relation_column_id`：本表某 **relationship** 列 id（该列规范化后须为 `cardinality: one`）
- `target_column_id`：关联表上要读取的**物理列** id

**ListRows** 通过 `LEFT JOIN` 关联表在主查询中算出 lookup 值，写入对应 lookup 列名键；外键为空或关联缺失时为 JSON `null`。

HTTP 无需额外参数；只要表上存在 lookup 列，列表结果中会带上该虚拟列。

创建 lookup 列时使用 `typeId: "lookup"`（内置列类型 id）。

### relation_fk（物理外键列）

列类型 **relation_fk** 在 data 库中创建**真实 PG 列**，类型与目标引用列一致（默认目标表 `id` 的 uuid/int8 等）。`config`：

- `target_table_id`：目标表逻辑名（必填）
- `target_column_id`：目标表被引用列（可选，默认 `id`）
- `add_fk`：为 `true` 时追加 `FOREIGN KEY` 约束

写入时与普通物理列相同，直接传标量 FK 值。与 **relationship** 虚拟列配合：many 关系在子表上通常用 uuid/text 列 + relationship 定义；跨表非 id 引用（如 vendor `legacy_id`）用 relation_fk。

### formula / rollup（虚拟列，只读）

- **formula**：`config.expression` 支持 `{{columnName}}` 引用同表物理列，ListRows / Query 时 SQL 计算。
- **rollup**：在 relationship 上对子表字段聚合（count/sum 等），只读。

## 行读写 JSON 格式

创建、更新、ListRows 响应均使用**扁平行**：列名（或列 id）与 `id` 同级，标量值为原生 JSON 类型。

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "amount": 99.5,
  "vendor_ref": 1001,
  "vendor_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

创建行时可省略 `id`（由服务端生成）。适用于 `POST /v1/tables/{tableId}/rows`、`PATCH .../rows/{rowId}`、`POST .../rows:bulkUpsert`。

### 写入规则

| 列类型 | 能否写入 | 说明 |
|--------|----------|------|
| 标量（text/number/uuid/…） | 是 | 直接写 PG 列 |
| choice（ENUM） | 是 | 值为枚举字符串 |
| relation_fk | 是 | 写 FK 标量 |
| relationship | **否** | 虚拟列；many 关系通过**子表 link 列**关联，one 关系通过**本表 target_column_id 物理列** |
| lookup | **否**（单行 API） | 只读投影；应写 relationship 对应的 **FK 物理列**（如 `goodsId`） |
| lookup | **是**（`rows:saveGraph`） | 可传 lookup 列名（如 `goods_name`），服务端按 schema 解析为 FK |
| formula / rollup | **否** | 只读计算 |

单行 API 写 lookup 列无效；`rows:saveGraph` 可传 lookup 列名或嵌套 one relationship 对象。

### 常用行 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v1/tables/{tableId}/rows` | 分页列表，支持 expand |
| POST | `/v1/tables/{tableId}/rows` | 创建单行 |
| PATCH | `/v1/tables/{tableId}/rows/{rowId}` | 更新单行 |
| DELETE | `/v1/tables/{tableId}/rows/{rowId}` | 删除 |
| POST | `/v1/tables/{tableId}/rows:query` | DSL 过滤查询 |
| POST | `/v1/tables/{tableId}/rows:bulkUpsert` | 批量 insert/update（**单表**，事务内） |
| POST | `/v1/tables/{tableId}/rows:saveGraph` | 嵌套保存主行 + relationship 子图（单事务） |
| POST | `/v1/tables/{tableId}/rows:bulkDelete` | 批量按 id 删除 |

## 嵌套保存 `rows:saveGraph`

`POST /v1/tables/{tableId}/rows:saveGraph` 在单事务内保存主行及 relationship 嵌套数据。JSON 形状由 **relationship cardinality** 决定（schema 驱动）：

| cardinality | payload 形状 | 行为 |
|-------------|--------------|------|
| **one**（如 `supply`） | object | 在关联表 create/update，写入本表 FK 列 |
| **many**（如 `items`） | array | upsert 子行，自动填充 link 列 |

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
- **many 数组**（如 `items`）：upsert 子行；默认 merge（只增改）。整单替换子行时加 `"_sync": { "items": "replace" }`。
- **lookup 列名**（如 `goods_name`）：按 schema 查已有行并写 FK（不 create）。
- **lookup vs one 嵌套**：`supply: { "code": "..." }` 创建关联行；`supply_code: "..."` 仅引用已有行。显式 FK 列优先。
- **formula / rollup**：忽略。

**响应**与请求同形，在对应位置回填 `id` 及解析出的 FK（如 `supply_id`）：

```json
{
  "id": "...",
  "order_remark": "rush",
  "supply_id": "...",
  "supply": { "id": "...", "code": "SUP-001" },
  "items": [
    { "id": "...", "qty": 2, "goods_id": "...", "goods": { "id": "...", "name": "Apple" } }
  ]
}
```

## 常用命令汇总

- **迁移 schema**（meta + data 双库）

  ```bash
  make migrate
  ```

- **启动服务**

  ```bash
  make run
  ```

- **构建 Docker 镜像**

  ```bash
  make docker-build
  ```
