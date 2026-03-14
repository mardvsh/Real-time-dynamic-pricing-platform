package internal

import (
	"context"
	"errors"
	"net/http"

	"dynamic-pricing-platform/pkg/observability"
	postgrespkg "dynamic-pricing-platform/pkg/postgres"
	redispkg "dynamic-pricing-platform/pkg/redis"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	redisv9 "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Product struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	BasePrice   float64 `json:"base_price"`
	DynamicPrice float64 `json:"dynamic_price"`
}

type App struct {
	logger *zap.Logger
	db     *pgxpool.Pool
	rdb    *redisv9.Client
	engine *gin.Engine
}

func New(ctx context.Context, logger *zap.Logger, pgDSN, redisAddr string) (*App, error) {
	db, err := postgrespkg.NewPool(ctx, pgDSN)
	if err != nil {
		return nil, err
	}
	rdb := redispkg.NewClient(redisAddr, "", 0)

	engine := gin.New()
	engine.Use(gin.Recovery(), observability.GinMetricsMiddleware("catalog-service"))

	a := &App{logger: logger, db: db, rdb: rdb, engine: engine}
	a.routes()
	return a, nil
}

func (a *App) routes() {
	a.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "catalog-service"})
	})
	a.engine.GET("/products", a.listProducts)
	a.engine.GET("/products/:id", a.getProduct)
	a.engine.GET("/metrics", gin.WrapH(observability.MetricsHandler()))
}

func (a *App) listProducts(c *gin.Context) {
	rows, err := a.db.Query(c.Request.Context(), `
		SELECT id::text, name, category, base_price
		FROM products
		ORDER BY id
	`)
	if err != nil {
		a.logger.Error("query products", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query products failed"})
		return
	}
	defer rows.Close()

	products := make([]Product, 0)
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Category, &p.BasePrice); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "scan products failed"})
			return
		}
		p.DynamicPrice = a.dynamicPrice(c.Request.Context(), p.ID, p.BasePrice)
		products = append(products, p)
	}

	c.JSON(http.StatusOK, products)
}

func (a *App) getProduct(c *gin.Context) {
	productID := c.Param("id")
	var p Product
	err := a.db.QueryRow(c.Request.Context(), `
		SELECT id::text, name, category, base_price
		FROM products
		WHERE id::text = $1
	`, productID).Scan(&p.ID, &p.Name, &p.Category, &p.BasePrice)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}

	p.DynamicPrice = a.dynamicPrice(c.Request.Context(), p.ID, p.BasePrice)
	c.JSON(http.StatusOK, p)
}

func (a *App) dynamicPrice(ctx context.Context, productID string, fallback float64) float64 {
	price, err := redispkg.GetPrice(ctx, a.rdb, productID)
	if err != nil {
		if errors.Is(err, redisv9.Nil) {
			return fallback
		}
		a.logger.Warn("redis get price failed", zap.String("product_id", productID), zap.Error(err))
		return fallback
	}
	return price
}

func (a *App) Run(addr string) error {
	return a.engine.Run(addr)
}

func (a *App) Close() {
	a.db.Close()
	_ = a.rdb.Close()
}
