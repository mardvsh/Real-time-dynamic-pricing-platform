package main

import (
	"context"
	"log"
	"os"
	"strings"

	"dynamic-pricing-platform/pkg/observability"
	"dynamic-pricing-platform/services/behavior-service/internal"

	"go.uber.org/zap"
)

func main() {
	logger, err := observability.NewLogger("behavior-service")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = logger.Sync() }()

	mongoURI := getenv("MONGO_URI", "mongodb://mongo:27017")
	brokers := strings.Split(getenv("KAFKA_BROKERS", "kafka:9092"), ",")
	listenAddr := getenv("LISTEN_ADDR", ":8082")

	app, err := internal.New(context.Background(), logger, mongoURI, brokers)
	if err != nil {
		logger.Fatal("create app", zap.Error(err))
	}
	defer app.Close()

	if err := app.Run(listenAddr); err != nil {
		logger.Fatal("run app", zap.Error(err))
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
