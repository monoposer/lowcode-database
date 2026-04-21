package metrics

import (
	"sync"
	"time"
)

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
