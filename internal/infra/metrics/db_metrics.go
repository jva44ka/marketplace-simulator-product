package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type DbMetrics struct {
	requestsTotal          *prometheus.CounterVec
	requestDuration        *prometheus.HistogramVec
	optimisticLockFailures prometheus.Counter
}

func NewDbMetrics() *DbMetrics {
	return &DbMetrics{
		requestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "db_requests_total",
			Help:        "Total number of DB requests",
			ConstLabels: prometheus.Labels{"service": "product"},
		}, []string{"method", "status"}),
		requestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "db_request_duration_seconds",
			Help:        "Duration of DB requests in seconds",
			ConstLabels: prometheus.Labels{"service": "product"},
			Buckets:     []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		}, []string{"method", "status"}),
		optimisticLockFailures: promauto.NewCounter(prometheus.CounterOpts{
			Name: "products_db_optimistic_lock_failures_total",
			Help: "Total number of optimistic lock failures in product count updates",
		}),
	}
}

func (rm *DbMetrics) ReportRequest(method, status string, duration time.Duration) {
	rm.requestsTotal.WithLabelValues(method, status).Inc()
	rm.requestDuration.WithLabelValues(method, status).Observe(duration.Seconds())
}

func (rm *DbMetrics) ReportOptimisticLockFailure() {
	rm.optimisticLockFailures.Inc()
}
