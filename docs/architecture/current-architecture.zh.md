# 当前架构分析（不含 RBAC）

> 基于仓库现状（`cmd/server` 单进程、`Tenant ⊃ Base ⊃ Table`、`record` LIST 分区）。  
> **不讨论** 细粒度权限 / RBAC（产品刻意外包给 BFF）。  
> 运行时地图见 [system.md](system.md)；演进权衡见 [analysis.md](analysis.md)。

---

## 1. 一句话结论

这是一个 **自托管的 Postgres 低代码表引擎**：Meta 库存逻辑模型，Data 库以 JSONB 统一行表 `record`（按 `vt_id` LIST 分区）存业务行。对外只有 **`/v1/admin` + `/v1/data`**；公式/Lookup/Rollup 通过进程内 **`calc_queue`** 异步写回缓存。身份与路由以 **`X-Tenant-Id`（+ 可选 `X-Base-Id`）** 为界，认证仅可选 API Key。

---

## 2. 进程与拓扑

```
┌─────────────────────── cmd/server（唯一长驻进程）───────────────────────┐
│  CORS → RequestLog → authn? → chi NewHandler                             │
│       /v1/admin/*   Meta 读写 + Data DDL                                 │
│       /v1/data/*    行读写 / DSL / 导出 / 执行保存的 Query                 │
│  后台 goroutine：                                                        │
│       calc.Worker          轮询各 Data DSN 的 calc_queue (SKIP LOCKED)   │
│       worker.IndexMigrate  调和 lc_indexes.migrate_status                │
│       EventBus 订阅        → HTTP webhook 投递                           │
│  旁路：/health /openapi /swagger [/playground]                           │
└──────────────────────────────────────────────────────────────────────────┘

cmd/migrate   ← 一次性 CLI，对 meta + 各 data DSN apply SQL；服务不自动 migrate
```

| 组件 | 路径 | 说明 |
|------|------|------|
| HTTP | `cmd/server` + `internal/api` | 生产面只有 admin + data |
| Calc | `internal/service/calc` | **无** `/v1/worker` HTTP |
| Index | `internal/worker/index_migrate.go` | 后台 CONCURRENTLY 建/删索引 |
| Migrate | `cmd/migrate` | embed `migrations/{meta,data}` |

**含义**：运维边界 = 「一个二进制的多副本」。Admin DDL、大导出、calc 轮询共享同一 CPU/连接池；水平扩展 = N 个同构实例（calc 靠 `SKIP LOCKED` 防重复消费，但会重复唤醒）。

---

## 3. 数据面：双库 + 租户路由

### 3.1 Meta vs Data

| 平面 | 配置 | 内容 |
|------|------|------|
| **Meta** | `META_DATABASE_URL` | `tenants`、`lc_bases`、表/列/索引/关系/Query、column types、API Key、webhooks |
| **Data** | `tenants.data_dsn`（可另配 write/reads） | `record` / `link_ref` / `calc_queue`（+ dedicated 前缀表） |

Data 侧扩展（`pg_stat_statements`）与**行父表由运行时 DDL 创建**（`pkg/infra/postgres/virtual_records.go`）。`cmd/migrate` 只迁 meta。

### 3.2 身份层级

```
Tenant (X-Tenant-Id)          ← 隔离单位 + DSN / 连接池键
  └── Base (X-Base-Id)        ← 逻辑「应用/库」；缺省取该租户最早创建的 base
        └── Table (name)      ← API 路径里的 tableName = 逻辑名
              └── vt_id       ← 全局唯一；record LIST 分区键
```

Meta 主键示例：`lc_tables (tenant_id, base_id, name)`。  
行表 **`record` 不含 `base_id`**：隔离靠 `tenant_id` + 分区键 `vt_id`（每逻辑表一分区）。

### 3.3 请求路由

```
HTTP
  ├─ X-Tenant-Id → tenants 行 → DataPool / DataReadPool（按 DSN 复用，非按 tenant_id 硬隔离池）
  ├─ X-Base-Id   → 校验属于该 tenant；未传则 ResolveBaseID → 首个 base
  ├─ 写路径      → DataPool（主库）
  └─ 读路径      → DataReadPool（副本轮询）；X-Read-Consistency: strong 强制主库
```

`record_store`：

- `shared`：同 DSN 上公共 `record` / `link_ref` / `calc_queue`
- `dedicated`：同 DSN 上 `{tenant_id}_record` 等（租户级物理表前缀）

二者都是 **「租户不跨 DSN」**；跨库 JOIN 不做。

---

## 4. 行存储与计算

### 4.1 `record`（原名 virtual_records）

```sql
-- 示意：统一堆表，按 vt_id 过滤
record (
  record_id, tenant_id, vt_id,
  data jsonb, version,
  created_at, updated_at, …
  PRIMARY KEY (vt_id, record_id)
);
```

- 业务字段全在 `data`；公式/lookup/rollup **缓存**也写回 `data`（带 `version` 乐观锁）。
- 关联边在 `link_ref`；异步任务在 `calc_queue`（失败可进 `calc_dead_letter`）。
- 虚拟列 kind：`formula` / `link` / `lookup` / `rollup` — **无物理列**，定义在 `lc_columns.config`。

详见 [virtual-records.md](virtual-records.md)、[record-calc.md](record-calc.md)。

### 4.2 Calc 路径

```
行写入 / 依赖变更
  → enqueue calc_queue
  → calc.Worker Claim (SKIP LOCKED)
  → formula / lookup / rollup 引擎
  → UPDATE record.data（缓存单元格）
```

列表读缓存；详情可现算 DAG。**无跨行强一致**；多实例下无 calc「选主」，只有队列锁。

---

## 5. 应用分层与模块

```
handlers (internal/api)
    ↓  decode 到 service 域类型
LowcodeService
    ├─ schema    表/列/关系 + Data DDL
    ├─ catalog   类型列表、columnType、索引目录
    ├─ data      行 CRUD、DSL、bulk、export、search、入队 calc
    └─ platform  tenants/bases、API Key、Query 管理、webhooks、runtime
辅助：
    meta/     只读元数据门面
    shared/   Base（池/缓存/日志/总线）、Value、EmitEvent
    calc/     队列与引擎（由 main 启动，不挂在 facade 上）
引擎：dsl · query · formula · columntype · event
基建：pkg/infra/postgres · redis · pkg/platform/{authn,cache,ratelimit}
```

**JSON 契约**：类型与域同驻 `internal/service/{schema,catalog,data,platform,shared}`；**已无**独立 `apitype` / protobuf。

---

## 6. HTTP 面（生产）

| 前缀 | 职责 |
|------|------|
| `/v1/admin/*` | 租户/Base、表列索引、column-types、关系、ER、保存的 Query、webhooks、API Key、pg_stat、runtime |
| `/v1/data/*` | 行 CRUD、`:query` / `:bulkUpsert` / `:bulkDelete` / `:export` / `:search`；执行保存的 Query |

相关头：`X-Tenant-Id`、`X-Base-Id`、`X-Api-Key`、`X-Read-Consistency`、`X-Confirm-Dangerous`。

---

## 7. 事件

| 项 | 现状 |
|----|------|
| 总线 | `EVENT_BUS=memory`（默认）或 `redis` Stream |
| 发布 | `shared.EmitEvent`：事务后发布；失败丢弃（at-most-once） |
| 类型 | 行：`records.after.*`；内部 `metadata.*` → 总线上 `schema.*` |
| 出口 | Meta `lc_event_webhooks` + 进程内 dispatcher HTTP POST |
| **不做** | 库内 schema-audit / 事件日志表；外部靠 webhook 或自建 bus consumer |

多副本必须 `EVENT_BUS=redis`，否则 memory 总线无法跨实例。

---

## 8. 认证（非 RBAC）

- 可选 **API Key**（`API_KEY_REQUIRED` + Meta `lc_api_keys`）。
- 通过 Key 即视为该租户可调 admin + data；**没有**角色/资源 ACL。
- 细粒度授权预期由上游 BFF 完成（见 [roadmap.md](../roadmap.md)）。

---

## 9. 命名对照（读代码易混）

| 说法 | 实际含义 |
|------|----------|
| `tenant_id` / `X-Tenant-Id` | 隔离 + DSN 路由；历史文档里的 workspace/`ws_id` 已并入此概念 |
| `base_id` / `X-Base-Id` | 租户内逻辑库；只约束 Meta，不写在 `record` 行上 |
| API `tableName` / `Table.Id` | **逻辑表名** |
| `vt_id` | 分区键 / 表实例 ID（全局唯一） |
| `record` vs `virtual_records` | 物理表名是 `record`；代码里仍保留历史别名常量 |
| `/v1/admin` vs `/v1/data` | **模块划分**，不是独立进程边界 |

---

## 10. 设计取舍（不含权限）

### 做得干净的地方

1. **双库清晰**：目录与行流量、备份、连接池可分开扩。
2. **统一行模型**：一张（或 per-tenant 前缀）LIST 分区表 + JSONB，DDL 路径简单。
3. **读写分离无第二进程**：副本轮询在 `TenantManager`；强读显式 opt-in。
4. **事件可外送**：单机 memory / 多机 Redis Stream + webhook。
5. **契约与域同包**：少一层 `apitype` 漂移。

### 刻意简化 / 代价

1. **单进程爆炸半径**：DDL、导出、calc 同命运；水平扩 = 同构复制 + 多 poller。
2. **Meta 热路径依赖**：几乎每次行请求仍要读 Meta（vt_id、列规格）；Redis 只能缓，冷启动仍需 Meta 凭证。
3. **无事件 outbox**：提交成功但 publish 失败会丢；无内置审计表。
4. **副本延迟归调用方**：默认可读到滞后副本。
5. **运行时 DDL 与迁移文件分裂**：逻辑 schema 不在 `migrations/` 版本化。
6. **无跨租户/跨 DSN JOIN**；`record_store` 创建后不宜切换。

### 明确不在本版（roadmap）

插件 / WASM RPC、图查询与嵌套图写入、Choice/ENUM 产品 API、schema bundle 导入、calc 选主或 `-role` 拆分、事件至少一次投递 —— 见 [roadmap.md](../roadmap.md)。

---

## 11. 关键配置旋钮

| 类别 | 环境变量（节选） |
|------|------------------|
| 双库 | `META_DATABASE_URL`、`DATA_DSN_TEMPLATE` |
| HTTP | `HTTP_ADDR`、`API_KEY_REQUIRED`、`MAX_ROW`、`DDL_CONFIRM_REQUIRED` |
| 池 | `PG_MAX_CONNS`、`MAX_TENANT_DATA_POOLS`、`DEFAULT_TENANT_POOL_MAX_CONNS` |
| 缓存 | `REDIS_URL`、`CACHE_ENABLED`、`CACHE_TTL_SECONDS` |
| 事件 | `EVENT_BUS`、`EVENT_STREAM_KEY` |
| Calc / 索引 | `CALC_*`、`INDEX_BACKFILL_TIMEOUT_SEC` |
| 防护 | `MAX_SCAN_ROWS`、`MAX_BULK_ITEMS`、`MAX_EXPORT_ROWS` |

权威列表：`pkg/config/config.go`、`.env.example`。

---

## 12. 关键文件索引

| 主题 | 路径 |
|------|------|
| 入口 | `cmd/server/main.go`、`cmd/migrate/main.go` |
| 路由 | `internal/api/routes.go` |
| 门面 | `internal/service/lowcode_service.go` |
| Meta schema | `migrations/meta/000001_init.up.sql` |
| 连接 / 租户 | `pkg/infra/postgres/tenant_manager.go`、`tenant_pool.go`、`shard_pool.go` |
| 行表 DDL | `pkg/infra/postgres/virtual_records.go`、`record_store.go` |
| 上下文 | `pkg/tenant/context.go` |
| 模块索引 | [docs/modules/README.md](../modules/README.md) |

---

*文档生成自当前代码快照；若与实现冲突，以代码与 `migrations/` 为准。*
