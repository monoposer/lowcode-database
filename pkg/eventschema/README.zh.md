# eventschema — 事件投递 JSON Schema

**[English](README.md) | [中文](README.zh.md)**

可被其他项目引用的 **lowcode-database 事件投递契约**：统一 envelope 外壳 + 按 `type` 区分的 `data` payload。

Go 引用：

```go
import "github.com/monoposer/lowcode-database/pkg/eventschema"

raw := eventschema.EnvelopeSchema()
payload, ok := eventschema.PayloadSchema(eventschema.RecordsAfterInsert)
```

非 Go 项目可直接复制 [`schema/`](schema/) 下的 JSON Schema 文件。

服务运行时也可通过 HTTP 拉取：

- `GET /v1/admin/events/schemas` — 全部 payload schema
- `GET /v1/admin/events/envelope-schema` — envelope schema

---

## Envelope（统一外壳）

所有通过 HTTP POST 投递到 webhook `targetUrl` 的消息共用同一外层结构：

```json
{
  "type": "records.after.insert",
  "tenantId": "default",
  "tableId": "orders",
  "occurredAt": "2026-06-12T10:00:00.123456789Z",
  "data": { }
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `type` | 是 | 事件类型，见下表 |
| `tenantId` | 是 | 租户 id（与 `X-Tenant-Id` 一致） |
| `tableId` | 否 | 逻辑表名（有则填） |
| `occurredAt` | 是 | UTC 时间，RFC3339Nano |
| `data` | 是 | 与 `type` 对应的 payload，见各 JSON Schema |

Schema 文件：[`schema/envelope.json`](schema/envelope.json)

---

## 事件分类

### 1. 数据事件（`records.*`）

**租户业务表行变更**，用于搜索索引、缓存失效、下游同步等。

| type | 触发时机 | data 要点 |
|------|----------|-----------|
| `records.after.insert` | 单行创建 | `{ "row": { ... } }` |
| `records.after.update` | 单行更新 | `{ "row": { ... } }` |
| `records.after.delete` | 单行删除 | `{ "rowId": "..." }` |
| `records.after.bulkUpsert` | 批量 upsert | `{ "rows": [ ... ] }` |
| `records.after.bulkDelete` | 批量删除 | `{ "rowIds": [ ... ] }` |
| `records.after.bulkImport` | 导入完成 | `{ "rows": [ ... ], "insertedCount": N }` |

`row` / `rows` 的 key 为列逻辑名或 id；cell 值为 API `Value` 联合类型（`stringValue`、`numberValue` 等）。

示例：

```json
{
  "type": "records.after.insert",
  "tenantId": "default",
  "tableId": "orders",
  "occurredAt": "2026-06-12T10:00:00Z",
  "data": {
    "row": {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "amount": { "numberValue": 99.5 },
      "status": { "stringValue": "pending" }
    }
  }
}
```

### 2. 元数据事件（`metadata.*`）

**Meta 库 schema 变更**（表、列、枚举、索引、关系、DataSource），用于缓存失效、ER 图刷新、CI 触发等。

| type | 触发时机 | data 要点 |
|------|----------|-----------|
| `metadata.table.created` | 表创建 | `{ "table": { ... } }` |
| `metadata.table.deleted` | 表删除 | `{ "tableId": "..." }` |
| `metadata.table.renamed` | 表重命名 | `{ "oldName", "newName", "table" }` |
| `metadata.column.created` | 列创建 | `{ "column": { ... } }` |
| `metadata.column.updated` | 列更新 | `{ "column": { ... } }` |
| `metadata.column.deleted` | 列删除 | `{ "tableId", "columnId" }` |
| `metadata.choice.created` | ENUM 注册 | `{ "choice": { ... } }` |
| `metadata.choice.updated` | ENUM 更新 | `{ "choice": { ... } }` |
| `metadata.choice.deleted` | ENUM 删除 | `{ "choiceId": "..." }` |
| `metadata.relation.created` | 关系创建 | `{ "relation": { ... } }` |
| `metadata.relation.deleted` | 关系删除 | `{ "relationId": "..." }` |
| `metadata.index.created` | 索引创建 | `{ "index": { ... } }` |
| `metadata.index.deleted` | 索引删除 | `{ "tableId", "pgIndexName" }` |
| `metadata.datasource.created` | DataSource 创建 | `{ "dataSource": { ... } }` |
| `metadata.datasource.updated` | DataSource 更新 | `{ "dataSource": { ... } }` |
| `metadata.datasource.deleted` | DataSource 删除 | `{ "tableId", "name" }` |

嵌套资源（`table`、`column` 等）与 Admin API 返回 JSON 字段一致（camelCase）。

---

## 目录结构

```
pkg/eventschema/
├── README.md / README.zh.md
├── types.go       # 常量、分类、TypeInfo
├── envelope.go    # Envelope 结构体
├── schema.go      # embed 加载 API
└── schema/
    ├── envelope.json
    ├── records/
    │   └── after.*.json
    └── metadata/
        └── *.json
```

---

## 版本与兼容

- JSON Schema 草案：**2020-12**
- 新增事件：增加 `schema/<category>/<suffix>.json`（如 `records.after.insert` → `schema/records/after.insert.json`），在 `types.go` 注册，并与 `internal/event` 中 `type` 字符串保持一致
- 向后兼容：仅扩展 `data` 字段；删除字段需大版本说明
