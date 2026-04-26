package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type RequestMetrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
}

func NewRequestMetrics() *RequestMetrics {
	return &RequestMetrics{
		requestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "requests_total",
				Help:        "Total number of requests",
				ConstLabels: prometheus.Labels{"service": "product"},
			},
			[]string{"method", "code"},
		),
		requestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "request_duration_seconds",
				Help:        "Duration of requests in seconds",
				Buckets:     prometheus.DefBuckets,
				ConstLabels: prometheus.Labels{"service": "product"},
			},
			[]string{"method"},
		),
	}
}

func (rm *RequestMetrics) ReportRequestInfo(methodName string, code string, duration time.Duration) {
	rm.requestsTotal.WithLabelValues(methodName, code).Inc()
	rm.requestDuration.WithLabelValues(methodName).Observe(duration.Seconds())
}
