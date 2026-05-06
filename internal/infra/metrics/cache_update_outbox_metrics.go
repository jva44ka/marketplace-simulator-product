package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type CacheUpdateOutboxMetrics struct {
	recordsProcessed *prometheus.CounterVec
	recordsPending   prometheus.Gauge
	recordsDeadLetter prometheus.Gauge
	tickDuration     prometheus.Histogram
	recordAge        prometheus.Histogram
}

func NewCacheUpdateOutboxMetrics() *CacheUpdateOutboxMetrics {
	return &CacheUpdateOutboxMetrics{
		recordsProcessed: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "cache_update_outbox_records_processed_total",
			Help:        "Total number of cache-update outbox records processed",
			ConstLabels: prometheus.Labels{"service": "product"},
		}, []string{"status"}),
		recordsPending: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "cache_update_outbox_records_pending",
			Help:        "Current number of pending cache-update outbox records",
			ConstLabels: prometheus.Labels{"service": "product"},
		}),
		recordsDeadLetter: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "cache_update_outbox_records_dead_letter",
			Help:        "Current number of dead-letter cache-update outbox records",
			ConstLabels: prometheus.Labels{"service": "product"},
		}),
		tickDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "cache_update_outbox_tick_duration_seconds",
			Help:        "Duration of cache-update outbox job tick in seconds",
			Buckets:     prometheus.DefBuckets,
			ConstLabels: prometheus.Labels{"service": "product"},
		}),
		recordAge: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:        "cache_update_outbox_record_age_seconds",
			Help:        "Age of cache-update outbox record at processing time in seconds",
			Buckets:     []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
			ConstLabels: prometheus.Labels{"service": "product"},
		}),
	}
}

func (m *CacheUpdateOutboxMetrics) ReportProcessed(status string, count int) {
	m.recordsProcessed.WithLabelValues(status).Add(float64(count))
}

func (m *CacheUpdateOutboxMetrics) SetPending(count int64) {
	m.recordsPending.Set(float64(count))
}

func (m *CacheUpdateOutboxMetrics) SetDeadLetter(count int64) {
	m.recordsDeadLetter.Set(float64(count))
}

func (m *CacheUpdateOutboxMetrics) ReportTickDuration(d time.Duration) {
	m.tickDuration.Observe(d.Seconds())
}

func (m *CacheUpdateOutboxMetrics) ReportRecordAge(age time.Duration) {
	m.recordAge.Observe(age.Seconds())
}
