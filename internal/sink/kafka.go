package sink

import (
	"context"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

func deliverKafka(ctx context.Context, cfg map[string]any, body []byte) {
	brokers := configStringSlice(cfg, "brokers")
	topic := configString(cfg, "topic")
	if len(brokers) == 0 || topic == "" {
		return
	}
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		WriteTimeout: 10 * time.Second,
	}
	defer w.Close()

	if err := w.WriteMessages(ctx, kafka.Message{Value: body}); err != nil {
		log.Printf("sink kafka write %s: %v", topic, err)
	}
}
