package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type ProductEventOutboxMetrics struct {
	OutboxMetrics
	kafkaPublishDuration prometheus.Histogram
}

func NewProductEventOutboxMetrics() *ProductEventOutboxMetrics {
	return &ProductEventOutboxMetrics{
		OutboxMetrics: *NewOutboxMetrics("ProductEvent"),
		kafkaPublishDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "products_outbox_kafka_publish_duration_seconds",
			Help:        "Duration of Kafka batch publish in seconds",
			Buckets:     prometheus.DefBuckets,
			ConstLabels: prometheus.Labels{"service": "product", "job": "ProductEvent"},
		}),
	}
}

func (m *ProductEventOutboxMetrics) ReportKafkaPublishDuration(d time.Duration) {
	m.kafkaPublishDuration.Observe(d.Seconds())
}
