# Event delivery

> 中文：[事件投递.md](事件投递.md)

lowcode-database delivers JSON event envelopes via **async HTTP POST** after table/row or schema changes. Downstream can be a plain webhook or an **HTTP bridge** that forwards the POST to RabbitMQ, Kafka, SQS, SNS, Redis, etc.

Contract and JSON Schemas: [pkg/eventschema](../pkg/eventschema/README.md). Implementation: `internal/event/`.

## `lc_event_sinks` and `lc_webhooks`

Same table and capability; naming evolved over time:

| Stage | Table | API | Notes |
|-------|-------|-----|-------|
| Current | **`lc_event_sinks`** | **`/v1/admin/event-sinks`** | Single init migration (`000001_init.up.sql`); `sink` + `sink_config` + nullable `target_url` |

History: `lc_webhooks` → add `sink_type`/`sink_config` → rename to `lc_event_sinks`. Incremental migrations were squashed into init; fresh installs do not run 000002/000003.

**Current semantics:**

- Config table: **`lc_event_sinks`** (Admin API: `GET/POST/PATCH/DELETE /v1/admin/event-sinks`)
- Legacy **webhook subscription** = **event sink**; OpenAPI and JSON use `eventSink` / `eventSinks`
- Column **`sink`** is always `webhook` on write; **no native** Rabbit/Kafka/Redis/SQS/SNS drivers
- Column **`sink_config`** is legacy-only; if `target_url` is empty, `sink_config.url` may be used (must be `http://` or `https://`). Prefer **`target_url`** for new config
- Delivery URL: **`target_url`** (`http://` or `https://`, host required)

Migration SQL: `docker/postgres/migrations/meta/000001_init.up.sql` (`lc_event_sinks`, `lc_api_keys`, `lc_event_delivery_log`).

## Delivery flow

```
Write (Admin / Data API)
  → event.Bus async fan-out
  → match lc_event_sinks (enabled, eventTypes, tableFilter)
  → exponential backoff retry (EVENT_RETRY_MAX)
  → delivery/http POST target_url
     Content-Type: application/json
     optional headers (e.g. Authorization)
     optional secret → X-Lowcode-Signature (HMAC-SHA256 of body)
  → on exhaustion: lc_event_delivery_log (dead_letter) if EVENT_DLQ_ENABLED
```

- **At-least-once**: retries with backoff; consumers must be idempotent
- **Observability**: `GET /v1/admin/events/delivery-log`; Prometheus `lowcode_event_*` when `METRICS_BACKEND=prometheus`
- **eventTypes** empty = all `records.*`; non-empty = filter by type
- **tableFilter** non-empty = match logical table name only (`Table.Id` = name)

Env: `EVENT_RETRY_MAX` (default 3), `EVENT_RETRY_INITIAL_MS` (default 500), `EVENT_DLQ_ENABLED` (default true). See `.env.example`.

## Admin API example

```http
POST /v1/admin/event-sinks
X-Tenant-Id: default
Content-Type: application/json

{
  "name": "orders-to-bridge",
  "targetUrl": "https://event-bridge.internal/publish/kafka/orders-events",
  "tableFilter": "orders",
  "eventTypes": ["records.after.insert", "records.after.update"],
  "headers": {
    "Authorization": "Bearer <adapter-token>"
  },
  "secret": "<hmac-secret-for-adapter>",
  "enabled": true
}
```

`sink` and `sinkConfig` may be omitted; they are ignored on write except legacy `sinkConfig.url` fallback.

## How to set `target_url`

The service accepts **`http://` and `https://` only**. For message brokers, point **`target_url` at your HTTP adapter**; broker URLs, topics, and queues live **on the adapter** (env, path, or gateway config). Schemes like `kafka://` or `redis://` are rejected by the Admin API.

### 1. Direct HTTP webhook (default)

| Scenario | Example `target_url` |
|----------|----------------------|
| Custom BFF / SaaS | `https://api.example.com/hooks/lowcode` |
| Zapier / Make / n8n | Platform **webhook URL** |
| In-cluster service | `http://worker.default.svc.cluster.local:8080/events` |

Auth: sink `headers` + optional `secret` (verify `X-Lowcode-Signature`).

### 2. RabbitMQ

This service POSTs JSON → adapter → `basic.publish`.

| Approach | Example `target_url` | Notes |
|----------|----------------------|-------|
| Custom bridge | `https://bridge.example/rabbit/publish` | Adapter reads `AMQP_URL`, `exchange`, `routing_key` from env or path |
| Path per binding | `https://bridge.example/rabbit/exchanges/events/rk/orders` | One URL per binding; multiple sinks for multiple routes |
| Lambda / API GW | `https://xxx.execute-api.region.amazonaws.com/prod/rabbit` | Lambda publishes via AMQP client |

Adapter: parse envelope body, map routing key (e.g. table + event), handle its own retry/DLQ to the broker.

### 3. Kafka (REST proxy / HTTP gateway)

| Approach | Example `target_url` | Notes |
|----------|----------------------|-------|
| Confluent REST Proxy | `https://kafka-rest:8082/topics/lowcode-events` | POST body as record value; auth in sink `headers` |
| Custom HTTP→Kafka | `https://bridge.example/kafka/topics/lowcode-events` | Produce with `sarama`/`librdkafka`; key from `tableId` |
| Managed gateway | `https://gateway.example/v1/kafka/publish?topic=lowcode` | Gateway auth + forward |

Do **not** use `kafka://broker:9092/topic` — Admin API rejects it.

### 4. AWS SQS

SQS has no inbound HTTP URL; use **API Gateway / Lambda / container bridge** → `SendMessage`.

| Approach | Example `target_url` | Notes |
|----------|----------------------|-------|
| Single-queue bridge | `https://bridge.example/aws/sqs/lowcode-events` | Env: `AWS_REGION`, `QUEUE_URL`, IAM role |
| API Gateway | `https://abc123.execute-api.us-east-1.amazonaws.com/prod/sqs/lowcode` | Lambda: `SendMessage(QueueUrl, MessageBody=raw POST)` |

Multi-tenant: different path or bridge instance per `QUEUE_URL`.

### 5. AWS SNS

SNS is push-subscribe; it cannot be a POST target. Bridge receives POST → **`sns:Publish`**.

| Approach | Example `target_url` | Notes |
|----------|----------------------|-------|
| Publish bridge | `https://bridge.example/aws/sns/arn:aws:sns:region:acct:lowcode` | TopicArn in path or header |
| Unified gateway | `https://gateway.example/publish/sns/lowcode-metadata` | Gateway holds TopicArn / MessageAttributes |

For HTTP callbacks from SNS (SNS → your URL), that is the opposite direction from this event sink.

### 6. Redis (Stream / PubSub / List)

Redis is not HTTP; use a REST bridge or sidecar.

| Approach | Example `target_url` | Notes |
|----------|----------------------|-------|
| Custom bridge | `https://bridge.example/redis/xadd/lowcode-events` | Adapter: `XADD stream * payload <json>` |
| Upstash REST | `https://xxx.upstash.io/xadd/lowcode-events/*` | Token in sink `headers` (`Authorization: Bearer …`) |
| PubSub | `https://bridge.example/redis/publish/lowcode` | Adapter: `PUBLISH channel body` |

Fix stream/channel names in adapter config or URL path; do **not** use `redis://localhost:6379/0`.

## Minimum adapter contract

1. **Method**: `POST`
2. **Body**: raw lowcode envelope JSON ([pkg/eventschema](../pkg/eventschema/))
3. **Content-Type**: `application/json`
4. **Response**: 2xx means the adapter accepted the payload (broker durability is the adapter’s job; this service only checks HTTP status)
5. **Optional signature**: if sink `secret` is set, verify `X-Lowcode-Signature` (HMAC-SHA256 hex of body)

```go
func handleLowcode(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)
    // optional: verify X-Lowcode-Signature
    publishToYourBroker(body)
    w.WriteHeader(http.StatusOK)
}
```

## OpenAPI

OpenAPI (`internal/api/openapi/openapi.yaml`) uses **`EventSink`** / **`eventSinks`** / **`targetUrl`** / **`eventTypes`**, aligned with `internal/apiv1/types_event_sink.go`. `sinkConfig` is deprecated; broker integration uses HTTP adapters above.

## Related

- [internal/event/README.md](../internal/event/README.md)
- [pkg/eventschema](../pkg/eventschema/README.md)
