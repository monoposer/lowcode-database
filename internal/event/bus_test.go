package event

import (
	"context"
	"testing"
	"time"
)

func TestBusTypeSchemaAlias(t *testing.T) {
	if got := BusType(MetadataTableCreated); got != "schema.table.created" {
		t.Fatalf("got %q", got)
	}
	if got := BusType(RecordsAfterInsert); got != RecordsAfterInsert {
		t.Fatalf("got %q", got)
	}
}

func TestWebhookMatches(t *testing.T) {
	if !webhookMatches("", "records.after.insert") {
		t.Fatal("empty prefix should match")
	}
	if !webhookMatches("records.", "records.after.insert") {
		t.Fatal("prefix records. should match")
	}
	if webhookMatches("schema.", "records.after.insert") {
		t.Fatal("schema. should not match records")
	}
}

func TestMemoryBusPublish(t *testing.T) {
	b := NewMemoryBus()
	defer b.Close()
	got := make(chan Envelope, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = b.Subscribe(ctx, "g", "c", func(_ context.Context, e Envelope) error {
			got <- e
			return nil
		})
	}()
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		b.mu.RLock()
		n := len(b.subs)
		b.mu.RUnlock()
		if n > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := b.Publish(ctx, Envelope{Type: RecordsAfterInsert, TenantID: "t1"}); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-got:
		if e.Type != RecordsAfterInsert {
			t.Fatalf("got %+v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("handler not called")
	}
}
