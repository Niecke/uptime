-- +goose Up
CREATE UNIQUE INDEX UC_url ON endpoints(url);

-- +goose Down
DROP INDEX UC_url ON endpoints(url);