package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type OutboxMetrics struct {
	recordsProcessed *prometheus.CounterVec
	tickDuration     prometheus.Histogram
	recordAge        prometheus.Histogram
}

func NewOutboxMetrics(outboxName string) *OutboxMetrics {
	return &OutboxMetrics{
		recordsProcessed: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "outbox_records_processed_total",
			Help:        "Total number of outbox records processed",
			ConstLabels: prometheus.Labels{"service": "product", "job": outboxName},
		}, []string{"status"}),
		tickDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "outbox_tick_duration_seconds",
			Help:        "Duration of outbox job tick in seconds",
			Buckets:     prometheus.DefBuckets,
			ConstLabels: prometheus.Labels{"service": "product", "job": outboxName},
		}),
		recordAge: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "outbox_record_age_seconds",
			Help:        "Age of outbox record at processing time in seconds",
			Buckets:     []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
			ConstLabels: prometheus.Labels{"service": "product", "job": outboxName},
		}),
	}
}

func (m *OutboxMetrics) ReportProcessed(status string, count int) {
	m.recordsProcessed.WithLabelValues(status).Add(float64(count))
}

func (m *OutboxMetrics) ReportTickDuration(d time.Duration) {
	m.tickDuration.Observe(d.Seconds())
}

func (m *OutboxMetrics) ReportRecordAge(age time.Duration) {
	m.recordAge.Observe(age.Seconds())
}
