-- +goose Up
-- Hard-cutover development migration marker.
-- Baseline migrations now create privileges_json directly.
SELECT 1;

-- +goose Down
SELECT 1;
