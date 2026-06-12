package metrics_test

import (
	"context"
	"testing"
	"time"

	"github.com/monoposer/lowcode-database/internal/platform/metrics"
)

func TestRingBufferRollingAverage(t *testing.T) {
	r := metrics.NewRingBufferForTest(3)

	r.Add(100 * time.Millisecond)
	r.Add(200 * time.Millisecond)
	st := r.Stats()
	if st.Count != 2 || st.AvgDuration != 150*time.Millisecond {
		t.Fatalf("unexpected stats after 2 adds: %+v", st)
	}

	r.Add(300 * time.Millisecond)
	r.Add(400 * time.Millisecond) // evicts 100ms
	st = r.Stats()
	if st.Count != 3 {
		t.Fatalf("count = %d, want 3", st.Count)
	}
	wantAvg := (200*time.Millisecond + 300*time.Millisecond + 400*time.Millisecond) / 3
	if st.AvgDuration != wantAvg {
		t.Fatalf("avg = %v, want %v", st.AvgDuration, wantAvg)
	}
}

func TestPrometheusMetricsRecord(t *testing.T) {
	m := metrics.NewPrometheus(2)
	ctx := context.Background()
	m.Record(ctx, "t1", "ds1", 50*time.Millisecond, nil)
	m.Record(ctx, "t1", "ds1", 150*time.Millisecond, nil)

	st, err := m.Stats(ctx, "t1", "ds1")
	if err != nil {
		t.Fatal(err)
	}
	if st.Count != 2 || st.AvgDuration != 100*time.Millisecond {
		t.Fatalf("stats = %+v", st)
	}
}
