package sink

import (
	"context"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

func deliverRabbitMQ(ctx context.Context, cfg map[string]any, body []byte) {
	url := configString(cfg, "url")
	exchange := configString(cfg, "exchange")
	if url == "" || exchange == "" {
		return
	}
	routingKey := configString(cfg, "routingKey")

	conn, err := amqp.Dial(url)
	if err != nil {
		log.Printf("sink rabbitmq dial: %v", err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Printf("sink rabbitmq channel: %v", err)
		return
	}
	defer ch.Close()

	pub := amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	}
	if err := ch.PublishWithContext(ctx, exchange, routingKey, false, false, pub); err != nil {
		log.Printf("sink rabbitmq publish %s: %v", exchange, err)
	}
}
