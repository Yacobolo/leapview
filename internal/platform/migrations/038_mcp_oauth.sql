-- +goose Up

CREATE TABLE oauth_clients (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  redirect_uris_json TEXT NOT NULL,
  grant_types_json TEXT NOT NULL,
  response_types_json TEXT NOT NULL,
  scopes_json TEXT NOT NULL,
  audience_json TEXT NOT NULL,
  public INTEGER NOT NULL CHECK (public IN (0, 1)),
  secret_hash BLOB,
  token_endpoint_auth_method TEXT NOT NULL,
  principal_id TEXT REFERENCES principals(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE oauth_sessions (
  kind TEXT NOT NULL CHECK (kind IN ('authorize_code', 'access_token', 'refresh_token', 'pkce')),
  signature TEXT NOT NULL,
  request_id TEXT NOT NULL,
  request_json TEXT NOT NULL,
  access_signature TEXT NOT NULL DEFAULT '',
  active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (kind, signature)
);

CREATE INDEX oauth_sessions_request_idx
  ON oauth_sessions(request_id, kind);

CREATE TABLE oauth_client_assertions (
  jti TEXT PRIMARY KEY,
  expires_at TEXT NOT NULL
);

-- +goose Down

DROP TABLE oauth_client_assertions;
DROP INDEX oauth_sessions_request_idx;
DROP TABLE oauth_sessions;
DROP TABLE oauth_clients;
