package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"dynamic-pricing-platform/pkg/observability"
	"dynamic-pricing-platform/services/pricing-service/internal"

	"go.uber.org/zap"
)

func main() {
	logger, err := observability.NewLogger("pricing-service")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = logger.Sync() }()

	pgDSN := getenv("POSTGRES_DSN", "postgres://postgres:postgres@postgres:5432/dynamic_pricing?sslmode=disable")
	redisAddr := getenv("REDIS_ADDR", "redis:6379")
	mongoURI := getenv("MONGO_URI", "mongodb://mongo:27017")
	brokers := strings.Split(getenv("KAFKA_BROKERS", "kafka:9092"), ",")
	listenAddr := getenv("LISTEN_ADDR", ":8083")

	minMultiplier := parseFloatEnv("PRICING_MIN_MULTIPLIER", 0.70)
	maxMultiplier := parseFloatEnv("PRICING_MAX_MULTIPLIER", 1.50)
	coolingInterval := parseDurationEnv("PRICING_COOLING_INTERVAL", 15*time.Second)
	coolingStepPct := parseFloatEnv("PRICING_COOLING_STEP_PCT", 0.01)
	dynamicTTL := parseDurationEnv("PRICING_DYNAMIC_TTL", 30*time.Second)

	cfg := internal.PricingConfig{
		MinMultiplier:   minMultiplier,
		MaxMultiplier:   maxMultiplier,
		CoolingInterval: coolingInterval,
		CoolingStepPct:  coolingStepPct,
		DynamicPriceTTL: dynamicTTL,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app, err := internal.New(ctx, logger, pgDSN, redisAddr, mongoURI, brokers, cfg)
	if err != nil {
		logger.Fatal("create app", zap.Error(err))
	}
	defer app.Close()

	go func() {
		if err := app.StartConsumer(ctx, brokers); err != nil {
			logger.Fatal("consumer failed", zap.Error(err))
		}
	}()

	go app.StartAutoCooling(ctx)

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

func parseFloatEnv(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseDurationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
