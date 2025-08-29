// Package config - provides configuration for launching the app from .env
package config

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config -
type Config struct {
	DSN                 string
	AppPort             string
	KafkaBroker         string
	Topic               string
	DLQTopic            string
	LaunchMockGenerator bool
	CacheSize           int
}

// LoadSrvConfig -
func LoadSrvConfig(r http.Handler, appPort string) *http.Server {
	return &http.Server{
		Addr:         ":" + appPort,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

// GetConfig -
func GetConfig() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning while loading .env:", err)
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
		log.Fatalf("Failed to parse START_MOCK_PRODUCER from .env: %v", err)
	}

	cacheSize, err := strconv.ParseInt(os.Getenv("CACHE_SIZE"), 10, 0)
	if err != nil {
		log.Fatalf("Failed to parse CACHE_SIZE from .env: %v", err)
	}

	dlqTopic := os.Getenv("DLQ_TOPIC")
	switch dlqTopic {
	case "":
		log.Fatal("DLQ_TOPIC is not set in env")
	case topic:
		log.Fatal("DLQ_TOPIC cannot be equal to KAFKA_TOPIC")
	}

	return Config{
		DSN:                 dsn,
		AppPort:             port,
		KafkaBroker:         broker,
		Topic:               topic,
		DLQTopic:            dlqTopic,
		LaunchMockGenerator: mockStart,
		CacheSize:           int(cacheSize),
	}
}
