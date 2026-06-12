# 文档索引

| 文档 | 说明 |
|------|------|
| [技术架构.md](技术架构.md) | 权威技术架构：双库、请求链路、API 平面、目录结构、环境变量 |
| [架构分析.md](架构分析.md) | **本仓库**分层、域模块、双库数据流、apiv1 组织、优劣势与演进 |
| [事件投递.md](事件投递.md) / [event-delivery.md](event-delivery.md) | **`lc_event_sinks`**、HTTP 投递、重试/DLQ、Kafka/Rabbit/Redis/SQS/SNS 适配器 URL |
| [modules.md](modules.md) | 包级索引（英文简要说明） |

相关外部文档：

- [pkg/eventschema](../pkg/eventschema/README.zh.md) — 事件 envelope 与 JSON Schema
- [deploy/](../deploy/README.md) — Docker 部署与发布
- [AGENTS.md](../AGENTS.md) — Agent / 开发者速查
