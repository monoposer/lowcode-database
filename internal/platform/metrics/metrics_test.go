package metrics_test

import (
	"testing"

	"github.com/monoposer/lowcode-database/internal/platform/metrics"
)

func TestClampLimit(t *testing.T) {
	if got := metrics.ClampLimit(0); got != 100 {
		t.Fatalf("default: got %d", got)
	}
	if got := metrics.ClampLimit(50); got != 50 {
		t.Fatalf("passthrough: got %d", got)
	}
	if got := metrics.ClampLimit(9999); got != 500 {
		t.Fatalf("max: got %d", got)
	}
}
