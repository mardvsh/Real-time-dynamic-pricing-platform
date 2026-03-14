package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"dynamic-pricing-platform/pkg/events"
	"dynamic-pricing-platform/pkg/kafka"
	mongopkg "dynamic-pricing-platform/pkg/mongo"
	"dynamic-pricing-platform/pkg/observability"

	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type App struct {
	logger      *zap.Logger
	engine      *gin.Engine
	snapshots   *mongo.Collection
	priceEvents *mongo.Collection
}

func New(ctx context.Context, logger *zap.Logger, mongoURI string) (*App, error) {
	mongoClient, err := mongopkg.NewClient(ctx, mongoURI)
	if err != nil {
		return nil, err
	}

	engine := gin.New()
	engine.Use(gin.Recovery(), observability.GinMetricsMiddleware("analytics-service"))

	a := &App{
		logger:      logger,
		engine:      engine,
		snapshots:   mongoClient.Database("dynamic_pricing").Collection("analytics_snapshots"),
		priceEvents: mongoClient.Database("dynamic_pricing").Collection("analytics_price_updates"),
	}
	a.routes()
	return a, nil
}

func (a *App) routes() {
	a.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "analytics-service"})
	})
	a.engine.GET("/metrics", gin.WrapH(observability.MetricsHandler()))
}

func (a *App) StartConsumer(ctx context.Context, brokers []string) error {
	return kafka.Consume(ctx, kafka.Config{Brokers: brokers}, "analytics-service-group", []string{"analytics-events", "price-updates"}, a.handleMessage)
}

func (a *App) handleMessage(ctx context.Context, msg *sarama.ConsumerMessage) error {
	switch msg.Topic {
	case "analytics-events":
		var event events.AnalyticsEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return err
		}
		record := map[string]any{
			"type":      event.Type,
			"payload":   event.Payload,
			"timestamp": event.Timestamp,
			"ingestedAt": time.Now().UTC(),
		}
		_, err := a.snapshots.InsertOne(ctx, record)
		return err
	case "price-updates":
		var update events.PriceUpdateEvent
		if err := json.Unmarshal(msg.Value, &update); err != nil {
			return err
		}
		record := map[string]any{
			"product_id": update.ProductID,
			"old_price":  update.OldPrice,
			"new_price":  update.NewPrice,
			"reason":     update.Reason,
			"timestamp":  update.Timestamp,
			"ingestedAt": time.Now().UTC(),
		}
		_, err := a.priceEvents.InsertOne(ctx, record)
		return err
	default:
		return nil
	}
}

func (a *App) Run(addr string) error {
	return a.engine.Run(addr)
}
