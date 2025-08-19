package kafka

import (
	"context"
	"log"
	"orderservice/internal/service"
	"sync"
)

// StartConsumer initializes listening to Kafka messages, which will be forwarded to Service-layer
func StartConsumer(ctx context.Context, srv service.OrderService, broker, topic string, wg *sync.WaitGroup) {
	defer wg.Done()
	reader := NewKafkaReader(broker, topic)
	defer reader.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := reader.ReadMessage(ctx)
			if err != nil {
				log.Printf("Kafka read error: %v", err)
				continue
			}
			srv.AddNewOrder(&msg)
			reader.CommitMessages(ctx, msg)
		}

	}
}
