# Tenant data DB

**无 SQL migration 文件。** 物理表、PG ENUM、索引由服务在运行时 DDL（`internal/service/schema`、`catalog`）。

## PostgreSQL 版本

要求 **PostgreSQL 16+**。

## 扩展（Extensions）

| 扩展 | 说明 |
|------|------|
| **pgcrypto** | 不需要。`gen_random_uuid()` 自 PG 13 起为核心内置。 |
| **postgis** | 使用 `geometry` / `geography` / `point` 列时需要。见下文。 |

---

## 如何启用 PostGIS

PostGIS **不是** PostgreSQL 核心的一部分，需要两步：

1. **安装 PostGIS 软件**（提供 `postgis` extension 的二进制与 SQL 脚本）
2. **在每个 tenant data 库执行** `CREATE EXTENSION postgis;`

缺任一步，建 `geometry` 列时会报错 `type "geometry" does not exist` 或 `extension "postgis" is not available`。

### 本地 Docker（推荐）

`deploy/docker-compose.yml` 使用 **`postgis/postgis:16-3.5`**（替代 `postgres:16-alpine`）。  
首次 init 时 `docker/postgres/init/01-init-databases.sh` 会在 `lowcode_data` 上执行 `CREATE EXTENSION postgis`。

**已有旧 volume（之前用 plain postgres 镜像）**：PostGIS 装不进已有 data 目录，需重建：

```bash
make docker-down
docker volume rm lowcode-database_pg-data   # 或 compose 项目名对应 volume
make docker-up
```

或在不删库的情况下，换 PostGIS 镜像启动后手动：

```bash
docker exec -it lowcode-postgres psql -U postgres -d lowcode_data -c 'CREATE EXTENSION IF NOT EXISTS postgis;'
```

（若镜像仍是 `postgres:16-alpine`，上述 SQL 会失败。）

### 自建 PostgreSQL（Linux）

Debian/Ubuntu 示例：

```bash
sudo apt install postgresql-16-postgis-3
sudo systemctl restart postgresql
```

RHEL / Amazon Linux：安装对应 `postgis` / `postgis33_16` 包（名称因发行版而异）。

然后在每个 data 库：

```sql
\c lowcode_data
CREATE EXTENSION IF NOT EXISTS postgis;
SELECT PostGIS_Version();
```

### 云托管

| 平台 | 做法 |
|------|------|
| **AWS RDS / Aurora** | 参数组允许 `postgis`；`CREATE EXTENSION postgis;`（部分区域需 `postgis_raster` 等子扩展按需加） |
| **GCP Cloud SQL** | 实例启用 PostGIS 标志或使用支持 PostGIS 的版本；连接后 `CREATE EXTENSION postgis;` |
| **Azure Database for PostgreSQL** | 允许列表中加入 `POSTGIS`；`CREATE EXTENSION postgis;` |
| **Supabase / Neon 等** | 控制台或 SQL 编辑器执行 `CREATE EXTENSION postgis;`（若套餐支持） |

### 多租户：每个 data 库都要 enable

`lowcode_data` 只是默认租户库。通过 Admin API 创建新租户且使用 **独立 database** 时，需在该库的 DSN 对应库上同样执行：

```sql
CREATE EXTENSION IF NOT EXISTS postgis;
```

可在租户 provisioning 脚本或 `DATA_ADMIN_DATABASE_URL` 建库流程里自动化。

### 验证

```bash
psql "$DEFAULT_TENANT_DATA_DSN" -c "SELECT PostGIS_Version();"
```

API 侧列类型：`geometry`、`geography`、`point`（见 `internal/columntype/types.go`）。
