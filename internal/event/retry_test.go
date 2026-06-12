package event

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/monoposer/lowcode-database/internal/event/delivery"
)

type stubPublisher struct {
	failures int
	calls    int
}

func (s *stubPublisher) Push(_ context.Context, _ string, _ delivery.Message) error {
	s.calls++
	if s.calls <= s.failures {
		return errors.New("temporary")
	}
	return nil
}

func TestPushWithRetrySuccessAfterFailures(t *testing.T) {
	pub := &stubPublisher{failures: 2}
	cfg := DeliveryConfig{RetryMax: 3, RetryInitial: time.Millisecond}
	attempts, err := pushWithRetry(context.Background(), pub, "https://example.com/h", delivery.Message{Body: []byte(`{}`)}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("attempts=%d want 3", attempts)
	}
}

func TestPushWithRetryExhausted(t *testing.T) {
	pub := &stubPublisher{failures: 5}
	cfg := DeliveryConfig{RetryMax: 2, RetryInitial: time.Millisecond}
	_, err := pushWithRetry(context.Background(), pub, "https://example.com/h", delivery.Message{}, cfg)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeliveryConfigFromDefaults(t *testing.T) {
	cfg := DeliveryConfigFrom(0, 0, true, "prometheus")
	if cfg.RetryMax != 3 || !cfg.DLQEnabled || !cfg.MetricsEnabled {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}
