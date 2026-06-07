package postgres

import (
	"sync/atomic"
	"testing"
)

func TestPickReadDSNFallsBackToWrite(t *testing.T) {
	if got := pickReadDSN("write", nil, new(uint64), false); got != "write" {
		t.Fatalf("got %q", got)
	}
	if got := pickReadDSN("write", []string{"r1"}, new(uint64), true); got != "write" {
		t.Fatalf("strong read got %q", got)
	}
}

func TestPickReadDSNRoundRobin(t *testing.T) {
	var rr uint64
	reads := []string{"a", "b"}
	got := map[string]int{}
	for i := 0; i < 4; i++ {
		got[pickReadDSN("w", reads, &rr, false)]++
	}
	if got["a"] != 2 || got["b"] != 2 {
		t.Fatalf("rr=%v atomic=%d", got, atomic.LoadUint64(&rr))
	}
}

func TestNormalizeReadDSNsDropsWriteAndDupes(t *testing.T) {
	out := normalizeReadDSNs("w", []string{" w ", "r1", "r1", "w", ""})
	if len(out) != 1 || out[0] != "r1" {
		t.Fatalf("got %#v", out)
	}
}
