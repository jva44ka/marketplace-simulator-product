package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type CacheMetrics struct {
	operationsTotal   *prometheus.CounterVec
	operationDuration *prometheus.HistogramVec
}

func NewCacheMetrics() *CacheMetrics {
	return &CacheMetrics{
		operationsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "products_cache_operations_total",
			Help:        "Total number of cache operations",
			ConstLabels: prometheus.Labels{"service": "product"},
		}, []string{"operation", "result"}),
		operationDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "products_cache_operation_duration_seconds",
			Help:        "Duration of cache operations in seconds",
			Buckets:     []float64{.0001, .0005, .001, .005, .01, .025, .05, .1},
			ConstLabels: prometheus.Labels{"service": "product"},
		}, []string{"operation"}),
	}
}

// ReportOperation records one cache operation.
//
// operation: "get" | "set" | "delete"
// result for get:    "hit" | "miss" | "stale" | "error"
// result for set:    "success" | "error"
// result for delete: "success" | "error"
//
// Pass duration=0 when timing is not applicable (e.g. stale bypass).
func (m *CacheMetrics) ReportOperation(operation, result string, duration time.Duration) {
	m.operationsTotal.WithLabelValues(operation, result).Inc()
	if duration > 0 {
		m.operationDuration.WithLabelValues(operation).Observe(duration.Seconds())
	}
}
