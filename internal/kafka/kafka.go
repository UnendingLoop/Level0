package kafka

import (
	"orderservice/internal/cache"
	"time"

	"github.com/segmentio/kafka-go"
	"gorm.io/gorm"
)

// Kafka provides db-connection and access for managing cache-map
type Kafka struct {
	DB  *gorm.DB
	Map *cache.OrderMap
}

// NewKafkaWriter returns new Kafka writer in order to emulate messages from it
func NewKafkaWriter(broker, topic string) *kafka.Writer {
	return kafka.NewWriter(kafka.WriterConfig{
		Brokers:      []string{broker},
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: int(kafka.RequireOne),
		Async:        false,
	})
}

// NewKafkaReader returns a new Kafka reader
func NewKafkaReader(broker, topic string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{broker},
		Topic:       topic,
		GroupID:     "order-service",
		MinBytes:    10e3,
		MaxBytes:    10e6,
		StartOffset: kafka.FirstOffset,
		MaxWait:     1 * time.Second,
	})
}
