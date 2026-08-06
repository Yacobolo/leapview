-- +goose Up
CREATE TABLE managed_data_multipart_claims (
  sha256 TEXT PRIMARY KEY,
  owner_id TEXT NOT NULL,
  lease_generation INTEGER NOT NULL DEFAULT 1 CHECK (lease_generation > 0),
  lease_until TEXT NOT NULL
);

-- +goose Down
DROP TABLE managed_data_multipart_claims;
