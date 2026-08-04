-- +goose Up
ALTER TABLE api_idempotency_records ADD COLUMN lease_generation INTEGER NOT NULL DEFAULT 1;
ALTER TABLE api_idempotency_records ADD COLUMN owner_session TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE api_idempotency_records DROP COLUMN owner_session;
ALTER TABLE api_idempotency_records DROP COLUMN lease_generation;
