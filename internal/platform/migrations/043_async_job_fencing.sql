-- +goose Up
ALTER TABLE api_async_jobs ADD COLUMN lease_generation INTEGER NOT NULL DEFAULT 0 CHECK(lease_generation >= 0);

-- +goose Down
ALTER TABLE api_async_jobs DROP COLUMN lease_generation;
