package metrics

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"github.com/monoposer/lowcode-database/internal/config"
	"strconv"
	"strings"
	"sync"
	"time"
)

// New returns a DataSourceMetrics backend from config.
func New(cfg *config.Config, rdb *redis.Client) DataSourceMetrics {
	switch strings.ToLower(cfg.MetricsBackend) {
	case "redis":
		if rdb == nil {
			return Noop{}
		}
		return NewRedis(rdb, cfg.MetricsWindowSize)
	case "prometheus":
		return NewPrometheus(cfg.MetricsWindowSize)
	default:
		return Noop{}
	}
}

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

// Redis stores last N query durations per data source in a Redis list.
type Redis struct {
	client *redis.Client
	window int
}

func NewRedis(client *redis.Client, window int) *Redis {
	if window <= 0 {
		window = 100
	}
	return &Redis{client: client, window: window}
}

func (m *Redis) key(tenantID, dataSourceID string) string {
	return fmt.Sprintf("lc:metrics:ds:%s:%s", tenantID, dataSourceID)
}

func (m *Redis) Record(ctx context.Context, tenantID, dataSourceID string, duration time.Duration, _ error) {
	key := m.key(tenantID, dataSourceID)
	us := strconv.FormatInt(duration.Microseconds(), 10)
	pipe := m.client.Pipeline()
	pipe.LPush(ctx, key, us)
	pipe.LTrim(ctx, key, 0, int64(m.window-1))
	pipe.Expire(ctx, key, 7*24*time.Hour)
	_, _ = pipe.Exec(ctx)
}

func (m *Redis) Stats(ctx context.Context, tenantID, dataSourceID string) (QueryStats, error) {
	vals, err := m.client.LRange(ctx, m.key(tenantID, dataSourceID), 0, int64(m.window-1)).Result()
	if err != nil {
		return QueryStats{}, fmt.Errorf("redis lrange: %w", err)
	}
	if len(vals) == 0 {
		return QueryStats{}, nil
	}
	var sum int64
	for _, v := range vals {
		us, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			continue
		}
		sum += us
	}
	return QueryStats{
		AvgDuration: time.Duration(sum/int64(len(vals))) * time.Microsecond,
		Count:       len(vals),
	}, nil
}

// ringBuffer keeps the last N durations for rolling average.
type ringBuffer struct {
	mu    sync.Mutex
	buf   []time.Duration
	size  int
	idx   int
	count int
}

func newRingBuffer(size int) *ringBuffer {
	if size <= 0 {
		size = 100
	}
	return &ringBuffer{buf: make([]time.Duration, size), size: size}
}

// NewRingBufferForTest exposes ring buffer for unit tests.
func NewRingBufferForTest(size int) *testRing {
	return &testRing{r: newRingBuffer(size)}
}

type testRing struct{ r *ringBuffer }

func (t *testRing) Add(d time.Duration) { t.r.add(d) }
func (t *testRing) Stats() QueryStats   { return t.r.stats() }

func (r *ringBuffer) add(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.idx] = d
	r.idx = (r.idx + 1) % r.size
	if r.count < r.size {
		r.count++
	}
}

func (r *ringBuffer) stats() QueryStats {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.count == 0 {
		return QueryStats{}
	}
	var sum time.Duration
	for i := 0; i < r.count; i++ {
		sum += r.buf[i]
	}
	return QueryStats{
		AvgDuration: sum / time.Duration(r.count),
		Count:       r.count,
	}
}
