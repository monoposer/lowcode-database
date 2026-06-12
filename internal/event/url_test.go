package event_test

import (
	"testing"

	"github.com/monoposer/lowcode-database/internal/event"
)

func TestResolveDeliveryURL(t *testing.T) {
	cases := []struct {
		name    string
		target  string
		cfg     map[string]any
		want    string
		wantErr bool
	}{
		{
			name:   "https passthrough",
			target: "https://example.com/hook",
			want:   "https://example.com/hook",
		},
		{
			name:   "http passthrough",
			target: "http://127.0.0.1:8080/hook",
			want:   "http://127.0.0.1:8080/hook",
		},
		{
			name: "legacy sink_config url",
			cfg: map[string]any{
				"url": "https://legacy.example/hook",
			},
			want: "https://legacy.example/hook",
		},
		{
			name:    "reject kafka scheme",
			target:  "kafka://localhost:9092/topic",
			wantErr: true,
		},
		{
			name:    "missing url",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := event.ResolveDeliveryURL(event.SinkWebhook, tc.target, tc.cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
			if err := event.ValidateDeliveryURL(got); err != nil {
				t.Fatalf("validate: %v", err)
			}
		})
	}
}

func TestSinkFromURL(t *testing.T) {
	if event.SinkFromURL("kafka://localhost:9092/topic") != event.SinkWebhook {
		t.Fatal("expected webhook")
	}
	if event.SinkFromURL("https://x") != event.SinkWebhook {
		t.Fatal("expected webhook")
	}
}

func TestValidateSinkConfig(t *testing.T) {
	if err := event.ValidateSinkConfig(event.SinkWebhook, "https://example.com/hook", nil); err != nil {
		t.Fatal(err)
	}
	if err := event.ValidateSinkConfig(event.SinkWebhook, "", nil); err == nil {
		t.Fatal("expected error")
	}
	if err := event.ValidateDeliveryURL("redis://localhost:6379/0"); err == nil {
		t.Fatal("expected error for redis scheme")
	}
}

func TestValidEventType(t *testing.T) {
	if !event.ValidEventType(event.MetadataTableCreated) {
		t.Fatal("expected metadata.table.created valid")
	}
	if event.ValidEventType("unknown.event") {
		t.Fatal("expected unknown invalid")
	}
}
