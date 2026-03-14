package main

import (
	"context"
	"log"
	"os"
	"strings"

	"dynamic-pricing-platform/pkg/observability"
	"dynamic-pricing-platform/services/analytics-service/internal"

	"go.uber.org/zap"
)

func main() {
	logger, err := observability.NewLogger("analytics-service")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = logger.Sync() }()

	mongoURI := getenv("MONGO_URI", "mongodb://mongo:27017")
	brokers := strings.Split(getenv("KAFKA_BROKERS", "kafka:9092"), ",")
	listenAddr := getenv("LISTEN_ADDR", ":8084")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := internal.New(ctx, logger, mongoURI)
	if err != nil {
		logger.Fatal("create app", zap.Error(err))
	}

	go func() {
		if err := app.StartConsumer(ctx, brokers); err != nil {
			logger.Fatal("consumer failed", zap.Error(err))
		}
	}()

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
