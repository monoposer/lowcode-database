package sink

import "testing"

func TestValidateCreate(t *testing.T) {
	tests := []struct {
		name      string
		sinkType  string
		targetURL string
		config    map[string]any
		wantErr   bool
	}{
		{
			name:      "webhook requires url",
			sinkType:  TypeWebhook,
			targetURL: "https://example.com/hook",
		},
		{
			name:     "redis stream",
			sinkType: TypeRedis,
			config: map[string]any{
				"url":    "redis://localhost:6379/0",
				"mode":   "stream",
				"stream": "lc-events",
			},
		},
		{
			name:     "redis pubsub",
			sinkType: TypeRedis,
			config: map[string]any{
				"url":     "redis://localhost:6379/0",
				"mode":    "pubsub",
				"channel": "lc-events",
			},
		},
		{
			name:     "rabbitmq",
			sinkType: TypeRabbitMQ,
			config: map[string]any{
				"url":      "amqp://guest:guest@localhost:5672/",
				"exchange": "lc.events",
			},
		},
		{
			name:     "kafka",
			sinkType: TypeKafka,
			config: map[string]any{
				"brokers": []any{"localhost:9092"},
				"topic":   "lc-events",
			},
		},
		{
			name:     "webhook missing url",
			sinkType: TypeWebhook,
			wantErr:  true,
		},
		{
			name:     "kafka missing topic",
			sinkType: TypeKafka,
			config: map[string]any{
				"brokers": []any{"localhost:9092"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCreate(tt.sinkType, tt.targetURL, tt.config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
