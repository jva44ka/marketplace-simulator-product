-- +goose Up
-- +goose StatementBegin
ALTER TABLE products DROP COLUMN count_reserved;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE products ADD COLUMN count_reserved INT NOT NULL DEFAULT 0;
-- +goose StatementEnd
