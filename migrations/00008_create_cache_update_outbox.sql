-- +goose Up
CREATE TABLE outbox.cache_updates
(
    record_id              UUID                     NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    sku                    BIGINT                   NOT NULL,
    created_at             TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    retry_count            INT                      NOT NULL DEFAULT 0,
    is_dead_letter         BOOLEAN                  NOT NULL DEFAULT FALSE,
    marked_as_dead_letter_at TIMESTAMP WITH TIME ZONE,
    dead_letter_reason     TEXT
);

CREATE INDEX idx_cache_updates_pending ON outbox.cache_updates (sku, created_at)
    WHERE is_dead_letter = FALSE;

-- +goose Down
DROP TABLE outbox.cache_updates;
