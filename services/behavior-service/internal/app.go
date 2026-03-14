package internal

import (
	"context"
	"net/http"
	"time"

	"dynamic-pricing-platform/pkg/events"
	"dynamic-pricing-platform/pkg/kafka"
	"dynamic-pricing-platform/pkg/observability"
	mongopkg "dynamic-pricing-platform/pkg/mongo"

	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

type App struct {
	logger    *zap.Logger
	engine    *gin.Engine
	producer  sarama.SyncProducer
	collection *mongo.Collection
}

func New(ctx context.Context, logger *zap.Logger, mongoURI string, brokers []string) (*App, error) {
	mongoClient, err := mongopkg.NewClient(ctx, mongoURI)
	if err != nil {
		return nil, err
	}

	producer, err := kafka.NewSyncProducer(kafka.Config{Brokers: brokers})
	if err != nil {
		return nil, err
	}

	collection := mongoClient.Database("dynamic_pricing").Collection("user_events")
	idx := mongo.IndexModel{Keys: map[string]any{"timestamp": 1}}
	_, _ = collection.Indexes().CreateOne(ctx, idx, options.CreateIndexes().SetMaxTime(5*time.Second))

	engine := gin.New()
	engine.Use(gin.Recovery(), observability.GinMetricsMiddleware("behavior-service"))

	a := &App{logger: logger, engine: engine, producer: producer, collection: collection}
	a.routes()
	return a, nil
}

func (a *App) routes() {
	a.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "behavior-service"})
	})
	a.engine.POST("/events", a.ingestEvent)
	a.engine.GET("/metrics", gin.WrapH(observability.MetricsHandler()))
}

func (a *App) ingestEvent(c *gin.Context) {
	var payload events.UserEvent
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if !validEvent(payload.Event) || payload.UserID == "" || payload.ProductID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event data"})
		return
	}
	if payload.Timestamp.IsZero() {
		payload.Timestamp = time.Now().UTC()
	}

	if _, err := a.collection.InsertOne(c.Request.Context(), payload); err != nil {
		a.logger.Error("save user event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save event failed"})
		return
	}

	if err := kafka.PublishJSON(a.producer, "user-events", payload.ProductID, payload); err != nil {
		a.logger.Error("publish user-events", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "publish failed"})
		return
	}

	analytics := events.AnalyticsEvent{
		Type:      "user_event_received",
		Payload:   map[string]any{"event": payload.Event, "product_id": payload.ProductID, "user_id": payload.UserID},
		Timestamp: time.Now().UTC(),
	}
	if err := kafka.PublishJSON(a.producer, "analytics-events", payload.ProductID, analytics); err != nil {
		a.logger.Warn("publish analytics-events", zap.Error(err))
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "queued"})
}

func validEvent(event string) bool {
	switch event {
	case "view_product", "add_to_cart", "purchase":
		return true
	default:
		return false
	}
}

func (a *App) Run(addr string) error {
	return a.engine.Run(addr)
}

func (a *App) Close() {
	_ = a.producer.Close()
}
