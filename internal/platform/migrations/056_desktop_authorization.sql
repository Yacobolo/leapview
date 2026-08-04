-- +goose Up

CREATE TABLE desktop_authorization_codes (
  code_hash BLOB PRIMARY KEY,
  principal_id TEXT NOT NULL,
  client_id TEXT NOT NULL,
  instance_id TEXT NOT NULL,
  profile_id TEXT NOT NULL,
  redirect_uri TEXT NOT NULL,
  code_challenge TEXT NOT NULL,
  return_path TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  consumed_at TEXT,
  created_at TEXT NOT NULL
);

CREATE INDEX desktop_authorization_codes_expiry_idx
  ON desktop_authorization_codes(expires_at);

CREATE TABLE desktop_sessions (
  session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
  instance_id TEXT NOT NULL,
  profile_id TEXT NOT NULL,
  client_id TEXT NOT NULL,
  absolute_expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE INDEX desktop_sessions_profile_idx
  ON desktop_sessions(instance_id, profile_id);

-- +goose Down

DROP INDEX desktop_sessions_profile_idx;
DROP TABLE desktop_sessions;
DROP INDEX desktop_authorization_codes_expiry_idx;
DROP TABLE desktop_authorization_codes;
