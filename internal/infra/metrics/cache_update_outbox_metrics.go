package metrics

type CacheUpdateOutboxMetrics struct {
	OutboxMetrics
}

func NewCacheUpdateOutboxMetrics() *CacheUpdateOutboxMetrics {
	return &CacheUpdateOutboxMetrics{
		OutboxMetrics: *NewOutboxMetrics("CacheUpdate"),
	}
}
