# Event delivery

The event bus POSTs JSON envelopes to **`lc_event_sinks.target_url`** (HTTP only).

**Full guide:** [docs/event-delivery.md](../../docs/event-delivery.md) · [docs/事件投递.md](../../docs/事件投递.md) (Chinese)

## Quick reference

- Table: **`lc_event_sinks`** (defined in meta migration `000001_init.up.sql`)
- Admin API: `/v1/admin/event-sinks`
- `target_url` must be `http://` or `https://` with a host
- Column `sink` is always `webhook`; no native broker drivers in core
- For Kafka, RabbitMQ, Redis, SQS, SNS: deploy an **HTTP adapter** and point `targetUrl` at it
- Auth: per-sink `headers` + optional `secret` → `X-Lowcode-Signature` (HMAC-SHA256)

Implementation: `bus.go`, `publisher.go`, `delivery/http.go`.
