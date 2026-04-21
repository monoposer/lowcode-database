# Lowcode Database (Postgres + HTTP JSON)

一个基于 Postgres 的简易 low-code / nocode 表格服务，支持：

- **Type**：列类型定义
- **Table**：动态创建/删除逻辑表
- **Column**：按列增删改（底层 ALTER TABLE）
- **Row/Cell**：创建/更新/删除单行和批量行
- **Index**：按列创建/删除索引
- **Relationship**：虚拟列类型，支持一对多、多对一/一对一；ListRows 时可指定 `expand_column_ids` 带出子表/关联表数据
- **Webhook（类 NocoDB）**：行级变更后向配置的 URL 发送 HTTP POST（JSON），可订阅 `records.after.insert` 等事件；见下文
- **HTTP JSON API**：`net/http` + `ServeMux`，路径 `/v1/*`（无 gRPC）
- 支持 **单库模式** 与 **多租户（数据库级隔离）模式**

## 环境要求

- Go 1.22+
- protoc（Protocol Buffers 编译器）
- protoc 插件：`protoc-gen-go`（仅生成消息类型，不含 gRPC）
- 一个可访问的 Postgres 实例

## 安装 proto 相关插件（示例）

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

确保 `$GOBIN`（或者 `$GOPATH/bin`）在 `PATH` 中。

## 生成 protobuf 对应的 Go 代码

在项目根目录执行：

```bash
make proto
```

生成的代码会放到 `gen/` 目录下（与 `proto/` 中的包路径一致）。

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

- **HTTP**（JSON API + 静态页面）：`http://localhost:8080/`
- API 前缀：`/v1/`（例如 `GET /v1/tables`）
- **API 文档（OpenAPI 3 + Swagger UI）**：`http://localhost:8080/swagger/`

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

每个租户数据库都会独立创建自身的：

- `lc_types`
- `lc_tables`
- `lc_columns`
- `lc_indexes`
- `lc_webhooks`

## Webhook（NocoDB 风格）

在租户库中配置 `lc_webhooks`（通过 API 管理）后，系统在行数据变更后会 **异步** `POST` 到 `targetUrl`，请求体为 JSON：

```json
{
  "type": "records.after.insert",
  "tableId": "<逻辑表名>",
  "data": { "row": { "id": "...", "cells": { } } }
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

## 测试页面

项目内置了一个简单的 HTML 测试页：

- 路径：`static/index.html`
- 访问地址：`http://localhost:8080/`

功能：

- 创建 Table
- 为 Table 添加 Column
- 根据列信息生成表单并创建 Record（Row）

如果在多租户模式下测试，可以使用浏览器开发者工具或自行扩展该页面，在每次 `fetch` 请求中添加 `X-Tenant-Id` 头。

独立 **Playground**（AG Grid）：`playground/` 目录，`make playground`。

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

- `cardinality` 为 **many**：`cells[列id]` 为 JSON：`{ "rows": [ { "id", "cells" }, ... ] }`（与旧行为一致）。
- `cardinality` 为 **one**：`cells[列id]` 为单个对象 `{ "id", "cells" }`；无关联行时为 JSON `null`。

HTTP 示例：`GET /v1/tables/{table_id}/rows?expand_column_ids=col-uuid-1&expand_column_ids=col-uuid-2`

### expand_paths（点分路径）

**ListRows** 支持 `expand_paths`（或 `expandPaths`）：逗号分隔多条路径，每条为 **点分隔的列 id**，至少两段。第一段为当前表上的 relationship 列 id，最后一段为叶子列 id；中间段为下一层表上的 relationship 列 id。

- 解析失败（路径过短、深度超过 5、未知列等）会使整次 `ListRows` 返回错误。
- 对 **many** 关系，每一跳最多展开 **100** 条子行。

示例：`GET /v1/tables/{table_id}/rows?expand_paths=relToOrder.orderName`

结果写入 `cells`，**键为完整路径字符串**（与 `expand_column_ids` 的列 id 键不冲突）：`cells["relToOrder.orderName"]`。

### lookup（虚拟列）

列类型 **lookup**（虚拟列），用于在 **多对一 / 一对一** 的 relationship 上投影关联表某一物理列。`config`：

- `relation_column_id`：本表某 **relationship** 列 id（该列规范化后须为 `cardinality: one`）
- `target_column_id`：关联表上要读取的**物理列** id

**ListRows** 通过 `LEFT JOIN` 关联表在主查询中算出 lookup 值，写入 `cells[lookup列id]`；外键为空或关联缺失时为 JSON `null`。

HTTP 无需额外参数；只要表上存在 lookup 列，列表结果中会带上该虚拟列。

创建 lookup 列时使用 `typeId: "lookup"`（与 `lc_types` 中 seed 的 id 一致）。

## 常用命令汇总

- **生成 proto 对应 Go 代码**

  ```bash
  make proto
  ```

- **启动服务**

  ```bash
  make run
  ```

- **清理生成代码**

  ```bash
  make clean
  ```
