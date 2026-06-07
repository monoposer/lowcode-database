package telemetry

import "context"

// Provider is the hook surface for tracing and metrics (implementations are optional).
type Provider interface {
	StartSpan(ctx context.Context, operation string, attrs map[string]string) (context.Context, func())
	RecordHistogram(name string, value float64, labels map[string]string)
	IncCounter(name string, labels map[string]string)
}

// Noop is the default no-op implementation.
type Noop struct{}

func (Noop) StartSpan(ctx context.Context, _ string, _ map[string]string) (context.Context, func()) {
	return ctx, func() {}
}

func (Noop) RecordHistogram(_ string, _ float64, _ map[string]string) {}

func (Noop) IncCounter(_ string, _ map[string]string) {}
