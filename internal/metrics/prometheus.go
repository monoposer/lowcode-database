package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	promQueryDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "lowcode_datasource_query_duration_seconds",
		Help:    "Data source query latency in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"tenant_id", "datasource_id", "status"})

	promQueryAvg = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lowcode_datasource_query_avg_seconds",
		Help: "Rolling average latency of the last N data source queries",
	}, []string{"tenant_id", "datasource_id"})

	promQueryWindow = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "lowcode_datasource_query_window_count",
		Help: "Number of samples in the rolling average window",
	}, []string{"tenant_id", "datasource_id"})

	promQueryTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "lowcode_datasource_query_total",
		Help: "Total data source queries",
	}, []string{"tenant_id", "datasource_id", "status"})

	promRegisterOnce sync.Once
)

func registerPrometheus() {
	promRegisterOnce.Do(func() {
		prometheus.MustRegister(promQueryDuration, promQueryAvg, promQueryWindow, promQueryTotal)
	})
}

// Prometheus exposes histogram/counter metrics and keeps an in-memory rolling average.
type Prometheus struct {
	window int
	rings  sync.Map // key -> *ringBuffer
}

func NewPrometheus(window int) *Prometheus {
	registerPrometheus()
	if window <= 0 {
		window = 100
	}
	return &Prometheus{window: window}
}

func (m *Prometheus) ringKey(tenantID, dataSourceID string) string {
	return tenantID + ":" + dataSourceID
}

func (m *Prometheus) ringFor(tenantID, dataSourceID string) *ringBuffer {
	key := m.ringKey(tenantID, dataSourceID)
	if v, ok := m.rings.Load(key); ok {
		return v.(*ringBuffer)
	}
	r := newRingBuffer(m.window)
	actual, _ := m.rings.LoadOrStore(key, r)
	return actual.(*ringBuffer)
}

func (m *Prometheus) Record(_ context.Context, tenantID, dataSourceID string, duration time.Duration, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	labels := prometheus.Labels{
		"tenant_id":     tenantID,
		"datasource_id": dataSourceID,
		"status":        status,
	}
	promQueryDuration.With(labels).Observe(duration.Seconds())
	promQueryTotal.With(labels).Inc()

	r := m.ringFor(tenantID, dataSourceID)
	r.add(duration)
	st := r.stats()
	gaugeLabels := prometheus.Labels{"tenant_id": tenantID, "datasource_id": dataSourceID}
	promQueryAvg.With(gaugeLabels).Set(st.AvgDuration.Seconds())
	promQueryWindow.With(gaugeLabels).Set(float64(st.Count))
}

func (m *Prometheus) Stats(_ context.Context, tenantID, dataSourceID string) (QueryStats, error) {
	if v, ok := m.rings.Load(m.ringKey(tenantID, dataSourceID)); ok {
		return v.(*ringBuffer).stats(), nil
	}
	return QueryStats{}, nil
}
