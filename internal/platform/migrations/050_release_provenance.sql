-- +goose Up
ALTER TABLE api_releases
ADD COLUMN provenance_json TEXT NOT NULL DEFAULT '{}'
CHECK(json_valid(provenance_json));

-- +goose Down
ALTER TABLE api_releases DROP COLUMN provenance_json;
