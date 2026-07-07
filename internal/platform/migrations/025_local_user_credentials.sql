-- +goose Up
CREATE TABLE IF NOT EXISTS local_user_credentials (
  principal_id TEXT PRIMARY KEY REFERENCES principals(id) ON DELETE CASCADE,
  password_verifier TEXT NOT NULL,
  must_change_password INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  password_changed_at TEXT
);

-- +goose Down
DROP TABLE IF EXISTS local_user_credentials;
