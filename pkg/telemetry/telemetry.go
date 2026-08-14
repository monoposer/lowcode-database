// Package telemetry records traces and metrics via OpenTelemetry.
//
// The global TracerProvider / MeterProvider are no-op until Init installs an
// OTLP exporter (when OTEL_EXPORTER_OTLP_ENDPOINT or the traces/metrics
// variants are set).
package telemetry

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "github.com/monoposer/lowcode-database"

var (
	tracer = otel.Tracer(instrumentationName)
	meter  = otel.Meter(instrumentationName)

	instMu sync.Mutex
	hists  = map[string]metric.Float64Histogram{}
	ctrs   = map[string]metric.Int64Counter{}
)

// StartSpan starts an OTel span. The returned func ends it.
func StartSpan(ctx context.Context, operation string, attrs map[string]string) (context.Context, func()) {
	ctx, span := tracer.Start(ctx, operation, trace.WithAttributes(toAttrs(attrs)...))
	return ctx, func() { span.End() }
}

// RecordHistogram records a float histogram (e.g. duration in milliseconds).
func RecordHistogram(name string, value float64, labels map[string]string) {
	h, err := histogram(name)
	if err != nil {
		return
	}
	h.Record(context.Background(), value, metric.WithAttributes(toAttrs(labels)...))
}

// IncCounter adds 1 to a counter.
func IncCounter(name string, labels map[string]string) {
	c, err := counter(name)
	if err != nil {
		return
	}
	c.Add(context.Background(), 1, metric.WithAttributes(toAttrs(labels)...))
}

// Labels builds the usual tenant / table / operation attributes.
func Labels(tenantID, baseID, tableName, apiOperation string) map[string]string {
	out := map[string]string{}
	if tenantID != "" {
		out["tenant_id"] = tenantID
	}
	if baseID != "" {
		out["base_id"] = baseID
	}
	if tableName != "" {
		out["table_name"] = tableName
	}
	if apiOperation != "" {
		out["api_operation"] = apiOperation
	}
	return out
}

func resetInstruments() {
	instMu.Lock()
	defer instMu.Unlock()
	hists = map[string]metric.Float64Histogram{}
	ctrs = map[string]metric.Int64Counter{}
}

func histogram(name string) (metric.Float64Histogram, error) {
	instMu.Lock()
	defer instMu.Unlock()
	if h, ok := hists[name]; ok {
		return h, nil
	}
	h, err := meter.Float64Histogram(name)
	if err != nil {
		return nil, err
	}
	hists[name] = h
	return h, nil
}

func counter(name string) (metric.Int64Counter, error) {
	instMu.Lock()
	defer instMu.Unlock()
	if c, ok := ctrs[name]; ok {
		return c, nil
	}
	c, err := meter.Int64Counter(name)
	if err != nil {
		return nil, err
	}
	ctrs[name] = c
	return c, nil
}

func toAttrs(labels map[string]string) []attribute.KeyValue {
	if len(labels) == 0 {
		return nil
	}
	out := make([]attribute.KeyValue, 0, len(labels))
	for k, v := range labels {
		out = append(out, attribute.String(k, v))
	}
	return out
}
