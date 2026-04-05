-- +goose Up
-- +goose StatementBegin
CREATE TABLE outbox.product_events
(
    record_id                UUID                     DEFAULT uuid_generate_v4() PRIMARY KEY,
    key                      TEXT                                   NOT NULL,
    data                     JSONB                                  NOT NULL,
    created_at               TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    retry_count              INTEGER                  DEFAULT 0     NOT NULL,
    is_dead_letter           BOOLEAN                  DEFAULT FALSE NOT NULL,
    marked_as_dead_letter_at TIMESTAMP WITH TIME ZONE               NULL,
    dead_letter_reason       TEXT                                   NULL
);

CREATE INDEX idx_outbox_product_events_pending ON outbox.product_events (created_at);

CREATE INDEX idx_outbox_product_events_key ON outbox.product_events (key, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE outbox.product_events;
-- +goose StatementEnd