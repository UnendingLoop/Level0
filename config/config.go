package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config -
type Config struct {
	DSN                 string
	AppPort             string
	KafkaBroker         string
	Topic               string
	LaunchMockGenerator bool
}

// GetConfig -
func GetConfig() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set in env")
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		log.Fatal("APP_PORT is not set in env")
	}

	broker := os.Getenv("KAFKA_BROKER")
	if broker == "" {
		log.Fatal("KAFKA_BROKER is not set in env")
	}

	topic := os.Getenv("KAFKA_TOPIC")
	if topic == "" {
		log.Fatal("KAFKA_TOPIC is not set in env")
	}

	mockStart, err := strconv.ParseBool(os.Getenv("START_MOCK_PRODUCER"))
	if err != nil {
		log.Fatal("START_MOCK_PRODUCER has invalid format in env.")
	}
	return Config{DSN: dsn, AppPort: port, KafkaBroker: broker, Topic: topic, LaunchMockGenerator: mockStart}
}
