package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type MetricCollectorMetrics struct {
	recordsPending     *prometheus.GaugeVec
	recordsDeadLetter  *prometheus.GaugeVec
	acquiredConns      prometheus.Gauge
	idleConns          prometheus.Gauge
	totalConns         prometheus.Gauge
	maxConns           prometheus.Gauge
	avgAcquireDuration prometheus.Gauge
}

func NewMetricCollectorMetrics() *MetricCollectorMetrics {
	return &MetricCollectorMetrics{
		recordsPending: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "outbox_records_pending",
			Help:        "Current number of pending outbox records (not dead lettered)",
			ConstLabels: prometheus.Labels{"service": "product"},
		}, []string{"outbox"}),
		recordsDeadLetter: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name:        "outbox_records_dead_letter",
			Help:        "Current number of dead letter outbox records",
			ConstLabels: prometheus.Labels{"service": "product"},
		}, []string{"outbox"}),
		acquiredConns: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "db_pool_acquired_conns",
			Help:        "Number of currently acquired pool connections",
			ConstLabels: prometheus.Labels{"service": "product"},
		}),
		idleConns: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "db_pool_idle_conns",
			Help:        "Number of currently idle pool connections",
			ConstLabels: prometheus.Labels{"service": "product"},
		}),
		totalConns: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "db_pool_total_conns",
			Help:        "Total number of connections in the pool",
			ConstLabels: prometheus.Labels{"service": "product"},
		}),
		maxConns: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "db_pool_max_conns",
			Help:        "Maximum pool size (MaxConns config value)",
			ConstLabels: prometheus.Labels{"service": "product"},
		}),
		avgAcquireDuration: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "db_pool_avg_acquire_duration_seconds",
			Help:        "Average connection acquire duration over the last collection interval",
			ConstLabels: prometheus.Labels{"service": "product"},
		}),
	}
}

func (m *MetricCollectorMetrics) SetPending(outboxName string, count int64) {
	m.recordsPending.WithLabelValues(outboxName).Set(float64(count))
}

func (m *MetricCollectorMetrics) SetDeadLetter(outboxName string, count int64) {
	m.recordsDeadLetter.WithLabelValues(outboxName).Set(float64(count))
}

func (m *MetricCollectorMetrics) SetAcquiredConns(n int32) {
	m.acquiredConns.Set(float64(n))
}

func (m *MetricCollectorMetrics) SetIdleConns(n int32) {
	m.idleConns.Set(float64(n))
}

func (m *MetricCollectorMetrics) SetTotalConns(n int32) {
	m.totalConns.Set(float64(n))
}

func (m *MetricCollectorMetrics) SetMaxConns(n int32) {
	m.maxConns.Set(float64(n))
}

func (m *MetricCollectorMetrics) SetAvgAcquireDuration(d time.Duration) {
	m.avgAcquireDuration.Set(d.Seconds())
}
