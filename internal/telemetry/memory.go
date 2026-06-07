package telemetry

import (
	"context"
	"sort"
	"sync"
	"time"
)

// Memory is an in-process metrics registry for admin scrape and tests.
type Memory struct {
	mu       sync.Mutex
	counters map[string]float64
	hist     map[string]*histAgg
}

type histAgg struct {
	Count float64
	Sum   float64
	Max   float64
}

func NewMemory() *Memory {
	return &Memory{
		counters: make(map[string]float64),
		hist:     make(map[string]*histAgg),
	}
}

func metricKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := name
	for _, k := range keys {
		out += "," + k + "=" + labels[k]
	}
	return out
}

func (m *Memory) StartSpan(ctx context.Context, _ string, _ map[string]string) (context.Context, func()) {
	return ctx, func() {}
}

func (m *Memory) RecordHistogram(name string, value float64, labels map[string]string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	k := metricKey(name, labels)
	h := m.hist[k]
	if h == nil {
		h = &histAgg{}
		m.hist[k] = h
	}
	h.Count++
	h.Sum += value
	if value > h.Max {
		h.Max = value
	}
}

func (m *Memory) IncCounter(name string, labels map[string]string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[metricKey(name, labels)]++
}

// Snapshot is a JSON-friendly dump of counters and histograms.
type Snapshot struct {
	Counters    map[string]float64           `json:"counters"`
	Histograms  map[string]HistogramSnapshot `json:"histograms"`
	CollectedAt time.Time                    `json:"collectedAt"`
}

type HistogramSnapshot struct {
	Count float64 `json:"count"`
	Sum   float64 `json:"sum"`
	Max   float64 `json:"max"`
	Mean  float64 `json:"mean"`
}

func (m *Memory) Snapshot() Snapshot {
	if m == nil {
		return Snapshot{Counters: map[string]float64{}, Histograms: map[string]HistogramSnapshot{}}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	c := make(map[string]float64, len(m.counters))
	for k, v := range m.counters {
		c[k] = v
	}
	h := make(map[string]HistogramSnapshot, len(m.hist))
	for k, v := range m.hist {
		mean := 0.0
		if v.Count > 0 {
			mean = v.Sum / v.Count
		}
		h[k] = HistogramSnapshot{Count: v.Count, Sum: v.Sum, Max: v.Max, Mean: mean}
	}
	return Snapshot{Counters: c, Histograms: h, CollectedAt: time.Now().UTC()}
}

// Labels builds the standard Phase-0 metric tags.
func Labels(tenantID, baseID, tableID, apiOperation string) map[string]string {
	out := map[string]string{}
	if tenantID != "" {
		out["tenant_id"] = tenantID
	}
	if baseID != "" {
		out["base_id"] = baseID
	}
	if tableID != "" {
		out["table_id"] = tableID
	}
	if apiOperation != "" {
		out["api_operation"] = apiOperation
	}
	return out
}
