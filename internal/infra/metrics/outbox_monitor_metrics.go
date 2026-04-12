package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type OutboxMonitorMetrics struct {
	recordsPending    prometheus.Gauge
	recordsDeadLetter prometheus.Gauge
}

func NewOutboxMonitorMetrics() *OutboxMonitorMetrics {
	return &OutboxMonitorMetrics{
		recordsPending: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "outbox_records_pending",
			Help: "Current number of pending outbox records (not dead lettered)",
		}),
		recordsDeadLetter: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "outbox_records_dead_letter",
			Help: "Current number of dead letter outbox records",
		}),
	}
}

func (m *OutboxMonitorMetrics) SetPending(count int64) {
	m.recordsPending.Set(float64(count))
}

func (m *OutboxMonitorMetrics) SetDeadLetter(count int64) {
	m.recordsDeadLetter.Set(float64(count))
}
