-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA outbox;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP SCHEMA outbox;
-- +goose StatementEnd