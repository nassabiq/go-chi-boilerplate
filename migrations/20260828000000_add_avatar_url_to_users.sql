-- +goose Up
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(1024) NULL;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS avatar_url;

