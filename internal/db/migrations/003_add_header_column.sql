-- +goose Up
ALTER TABLE check_results ADD COLUMN headers TEXT;

-- +goose Down
ALTER TABLE check_results DROP COLUMN headers TEXT;