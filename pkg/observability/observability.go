package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var (
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"service", "method", "path", "status"})

	HTTPRequestLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "method", "path"})

	PricingRecalcDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "pricing_recalculation_duration_seconds",
		Help:    "Duration of pricing recalculation",
		Buckets: prometheus.DefBuckets,
	})
)

func NewLogger(service string) (*zap.Logger, error) {
	return zap.NewProduction(zap.Fields(zap.String("service", service)))
}

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

func GinMetricsMiddleware(service string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		HTTPRequests.WithLabelValues(service, c.Request.Method, path, status).Inc()
		HTTPRequestLatency.WithLabelValues(service, c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}
