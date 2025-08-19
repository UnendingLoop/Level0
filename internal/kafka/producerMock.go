package kafka

import (
	"bufio"
	"context"
	"log"
	"os"
	"time"

	"github.com/segmentio/kafka-go"
)

// EmulateMsgSending used to emulate real messages flow to test the app in real-time with real DB; mock json-data is read from file
func EmulateMsgSending(broker, topic string) {
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:      []string{broker},
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: int(kafka.RequireOne),
		Async:        false,
	})

	file, err := os.Open("./internal/kafka/mocks.json")
	if err != nil {
		log.Fatalf("Failed to open json-mocks file: %v\nExiting application.", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	counter := 0
	for scanner.Scan() {
		time.Sleep(1 * time.Second)
		counter++
		line := scanner.Bytes()
		err = writer.WriteMessages(context.Background(), kafka.Message{
			Value: line,
		})
		for err != nil {
			log.Printf("Failed to publish test order #%d: %v", counter, err)
			log.Printf("Retrying to send order #%d...", counter)
			time.Sleep(5 * time.Second)
			err = writer.WriteMessages(context.Background(), kafka.Message{
				Value: line,
			})
		}

		log.Printf("Order #%d published to Kafka", counter)
	}

}

func WaitKafkaReady(broker string) {
	for {
		conn, err := kafka.Dial("tcp", broker)
		if err == nil {
			conn.Close()
			break
		}
		log.Println("Kafka not ready, retrying in 5s...")
		time.Sleep(5 * time.Second)
	}
	time.Sleep(25 * time.Second)
}
