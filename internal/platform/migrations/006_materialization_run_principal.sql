-- +goose Up

ALTER TABLE materialization_job_runs
ADD COLUMN principal_id TEXT REFERENCES principals(id) ON DELETE SET NULL;
