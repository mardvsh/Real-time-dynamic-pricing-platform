package main

import (
	"context"
	"log"
	"os"

	"dynamic-pricing-platform/pkg/observability"
	"dynamic-pricing-platform/services/catalog-service/internal"

	"go.uber.org/zap"
)

func main() {
	logger, err := observability.NewLogger("catalog-service")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = logger.Sync() }()

	pgDSN := getenv("POSTGRES_DSN", "postgres://postgres:postgres@postgres:5432/dynamic_pricing?sslmode=disable")
	redisAddr := getenv("REDIS_ADDR", "redis:6379")
	listenAddr := getenv("LISTEN_ADDR", ":8081")

	app, err := internal.New(context.Background(), logger, pgDSN, redisAddr)
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
