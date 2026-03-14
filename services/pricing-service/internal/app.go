package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"dynamic-pricing-platform/pkg/events"
	"dynamic-pricing-platform/pkg/kafka"
	mongopkg "dynamic-pricing-platform/pkg/mongo"
	"dynamic-pricing-platform/pkg/observability"
	postgrespkg "dynamic-pricing-platform/pkg/postgres"
	redispkg "dynamic-pricing-platform/pkg/redis"

	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	redisv9 "github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type App struct {
	logger       *zap.Logger
	db           *pgxpool.Pool
	rdb          *redisv9.Client
	producer     sarama.SyncProducer
	priceHistory *mongo.Collection
	engine       *gin.Engine
	mu           sync.Mutex
	cfg          PricingConfig
}

type PricingConfig struct {
	MinMultiplier    float64
	MaxMultiplier    float64
	CoolingInterval  time.Duration
	CoolingStepPct   float64
	DynamicPriceTTL  time.Duration
}

func New(ctx context.Context, logger *zap.Logger, pgDSN, redisAddr, mongoURI string, brokers []string, cfg PricingConfig) (*App, error) {
	db, err := postgrespkg.NewPool(ctx, pgDSN)
	if err != nil {
		return nil, err
	}
	rdb := redispkg.NewClient(redisAddr, "", 0)

	mongoClient, err := mongopkg.NewClient(ctx, mongoURI)
	if err != nil {
		return nil, err
	}

	producer, err := kafka.NewSyncProducer(kafka.Config{Brokers: brokers})
	if err != nil {
		return nil, err
	}

	engine := gin.New()
	engine.Use(gin.Recovery(), observability.GinMetricsMiddleware("pricing-service"))

	a := &App{
		logger:       logger,
		db:           db,
		rdb:          rdb,
		producer:     producer,
		priceHistory: mongoClient.Database("dynamic_pricing").Collection("price_history"),
		engine:       engine,
		cfg:          cfg,
	}
	a.routes()
	return a, nil
}

func (a *App) routes() {
	a.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "pricing-service"})
	})
	a.engine.GET("/price/:id", a.getPrice)
	a.engine.GET("/metrics", gin.WrapH(observability.MetricsHandler()))
}

func (a *App) getPrice(c *gin.Context) {
	productID := c.Param("id")
	price, err := redispkg.GetPrice(c.Request.Context(), a.rdb, productID)
	if err != nil {
		if errors.Is(err, redisv9.Nil) {
			var basePrice float64
			err = a.db.QueryRow(c.Request.Context(), `SELECT base_price FROM products WHERE id::text = $1`, productID).Scan(&basePrice)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"product_id": productID, "price": basePrice, "source": "postgres_base_price"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cache read failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"product_id": productID, "price": price, "source": "redis_dynamic"})
}

func (a *App) StartConsumer(ctx context.Context, brokers []string) error {
	return kafka.Consume(ctx, kafka.Config{Brokers: brokers}, "pricing-service-group", []string{"user-events"}, a.handleUserEvent)
}

func (a *App) handleUserEvent(ctx context.Context, msg *sarama.ConsumerMessage) error {
	start := time.Now()
	defer observability.PricingRecalcDuration.Observe(time.Since(start).Seconds())

	var event events.UserEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		a.logger.Error("unmarshal user event", zap.Error(err))
		return err
	}

	if event.ProductID == "" {
		return fmt.Errorf("empty product_id")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	views, err := a.rdb.Incr(ctx, fmt.Sprintf("views:product:%s", event.ProductID)).Result()
	if err != nil {
		return fmt.Errorf("incr views: %w", err)
	}
	if views == 1 {
		_ = a.rdb.Expire(ctx, fmt.Sprintf("views:product:%s", event.ProductID), time.Minute).Err()
	}

	var basePrice float64
	err = a.db.QueryRow(ctx, `SELECT base_price FROM products WHERE id::text = $1`, event.ProductID).Scan(&basePrice)
	if err != nil {
		return fmt.Errorf("load product base price: %w", err)
	}

	oldPrice, err := redispkg.GetPrice(ctx, a.rdb, event.ProductID)
	if err != nil {
		if errors.Is(err, redisv9.Nil) {
			oldPrice = basePrice
		} else {
			return fmt.Errorf("get old price: %w", err)
		}
	}

	newPrice, reason := CalculateDynamicPrice(basePrice, int(views), event.Event)
	newPrice = clampByBase(basePrice, newPrice, a.cfg.MinMultiplier, a.cfg.MaxMultiplier)
	if err := redispkg.SetPrice(ctx, a.rdb, event.ProductID, newPrice, a.cfg.DynamicPriceTTL); err != nil {
		return fmt.Errorf("set dynamic price: %w", err)
	}

	update := events.PriceUpdateEvent{
		ProductID: event.ProductID,
		OldPrice:  oldPrice,
		NewPrice:  newPrice,
		Reason:    reason,
		Timestamp: time.Now().UTC(),
	}

	if _, err := a.priceHistory.InsertOne(ctx, update); err != nil {
		a.logger.Warn("save price history", zap.Error(err))
	}

	if err := kafka.PublishJSON(a.producer, "price-updates", event.ProductID, update); err != nil {
		a.logger.Warn("publish price update", zap.Error(err))
	}

	a.logger.Info("price recalculated",
		zap.String("product_id", event.ProductID),
		zap.Float64("old_price", oldPrice),
		zap.Float64("new_price", newPrice),
		zap.Int64("views", views),
		zap.String("reason", reason),
	)

	return nil
}

func (a *App) StartAutoCooling(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.CoolingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.applyAutoCooling(ctx)
		}
	}
}

func (a *App) applyAutoCooling(ctx context.Context) {
	rows, err := a.db.Query(ctx, `SELECT id::text, base_price FROM products`)
	if err != nil {
		a.logger.Warn("auto-cooling query products failed", zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var productID string
		var basePrice float64
		if err := rows.Scan(&productID, &basePrice); err != nil {
			a.logger.Warn("auto-cooling scan failed", zap.Error(err))
			continue
		}

		if err := a.coolDownProduct(ctx, productID, basePrice); err != nil {
			a.logger.Warn("auto-cooling product failed", zap.String("product_id", productID), zap.Error(err))
		}
	}
}

func (a *App) coolDownProduct(ctx context.Context, productID string, basePrice float64) error {
	viewsKey := fmt.Sprintf("views:product:%s", productID)
	views, err := a.rdb.Get(ctx, viewsKey).Int64()
	if err != nil {
		if errors.Is(err, redisv9.Nil) {
			views = 0
		} else {
			return fmt.Errorf("read views: %w", err)
		}
	}

	targetPrice, reason := CalculateDynamicPrice(basePrice, int(views), "view_product")
	targetPrice = clampByBase(basePrice, targetPrice, a.cfg.MinMultiplier, a.cfg.MaxMultiplier)

	currentPrice, err := redispkg.GetPrice(ctx, a.rdb, productID)
	if err != nil {
		if errors.Is(err, redisv9.Nil) {
			currentPrice = basePrice
		} else {
			return fmt.Errorf("read current price: %w", err)
		}
	}

	if currentPrice <= targetPrice {
		return nil
	}

	step := basePrice * a.cfg.CoolingStepPct
	if step <= 0 {
		step = basePrice * 0.01
	}
	newPrice := currentPrice - step
	if newPrice < targetPrice {
		newPrice = targetPrice
	}
	newPrice = clampByBase(basePrice, round2(newPrice), a.cfg.MinMultiplier, a.cfg.MaxMultiplier)

	if math.Abs(newPrice-currentPrice) < 0.001 {
		return nil
	}

	if err := redispkg.SetPrice(ctx, a.rdb, productID, newPrice, a.cfg.DynamicPriceTTL); err != nil {
		return fmt.Errorf("set cooled price: %w", err)
	}

	update := events.PriceUpdateEvent{
		ProductID: productID,
		OldPrice:  currentPrice,
		NewPrice:  newPrice,
		Reason:    "auto_cooling_" + reason,
		Timestamp: time.Now().UTC(),
	}

	if _, err := a.priceHistory.InsertOne(ctx, update); err != nil {
		a.logger.Warn("save auto-cooling history", zap.Error(err))
	}

	if err := kafka.PublishJSON(a.producer, "price-updates", productID, update); err != nil {
		a.logger.Warn("publish auto-cooling update", zap.Error(err))
	}

	return nil
}

func (a *App) Run(addr string) error {
	return a.engine.Run(addr)
}

func (a *App) Close() {
	a.db.Close()
	_ = a.rdb.Close()
	_ = a.producer.Close()
}

func ParsePrice(raw string) (float64, error) {
	return strconv.ParseFloat(raw, 64)
}

func clampByBase(basePrice, price, minMultiplier, maxMultiplier float64) float64 {
	minPrice := basePrice * minMultiplier
	maxPrice := basePrice * maxMultiplier
	if price < minPrice {
		return round2(minPrice)
	}
	if price > maxPrice {
		return round2(maxPrice)
	}
	return round2(price)
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
