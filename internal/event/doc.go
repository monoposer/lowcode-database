// Package event implements the async event bus and HTTP webhook delivery to lc_event_sinks.
//
//   - types.go — DeliveryConfig, event types, sink config helpers
//   - bus.go — fan-out + HTTP delivery routing
//   - delivery_log.go — retry with backoff and dead-letter log writes
//   - url.go, schemas.go — URL helpers + Admin schema/metrics APIs
//   - delivery/ — HTTP driver (http.go), driver interface
package event
