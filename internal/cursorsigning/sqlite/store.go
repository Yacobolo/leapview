// Package sqlite loads the cursor signing key ring from durable instance state.
package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"

	"github.com/Yacobolo/libredash/internal/cursorsigning"
)

func Configure(ctx context.Context, db *sql.DB) error {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return fmt.Errorf("generate cursor signing key: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO api_cursor_signing_keys(key_id, secret, active, created_at)
		SELECT 'v1', ?, 1, ? WHERE NOT EXISTS (SELECT 1 FROM api_cursor_signing_keys WHERE active = 1)`, secret, now); err != nil {
		return fmt.Errorf("create cursor signing key: %w", err)
	}
	rows, err := db.QueryContext(ctx, `SELECT key_id, secret, active FROM api_cursor_signing_keys WHERE retired_at IS NULL ORDER BY created_at, key_id`)
	if err != nil {
		return fmt.Errorf("list cursor signing keys: %w", err)
	}
	defer rows.Close()
	keys := map[string][]byte{}
	current := ""
	for rows.Next() {
		var id string
		var key []byte
		var active bool
		if err := rows.Scan(&id, &key, &active); err != nil {
			return err
		}
		keys[id] = append([]byte(nil), key...)
		if active {
			current = id
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return cursorsigning.Configure(current, keys)
}
