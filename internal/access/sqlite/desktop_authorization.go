package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/flidai/leapview/internal/access/desktopauth"
)

func (r *Repository) StoreAuthorizationCode(
	ctx context.Context,
	code desktopauth.AuthorizationCode,
) error {
	if r == nil || r.root == nil {
		return fmt.Errorf("desktop authorization database is required")
	}
	if _, err := r.root.ExecContext(ctx, `
DELETE FROM desktop_authorization_codes
WHERE expires_at <= ? OR consumed_at IS NOT NULL
`, code.CreatedAt.Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("clean desktop authorization codes: %w", err)
	}
	if _, err := r.root.ExecContext(ctx, `
INSERT INTO desktop_authorization_codes (
  code_hash, principal_id, client_id, instance_id, profile_id,
  redirect_uri, code_challenge, return_path, expires_at, consumed_at, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?)
`, code.CodeHash[:], code.PrincipalID, code.ClientID, code.InstanceID,
		code.ProfileID, code.RedirectURI, code.CodeChallenge, code.ReturnPath,
		code.ExpiresAt.Format(time.RFC3339Nano),
		code.CreatedAt.Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return nil
}

func (r *Repository) ConsumeAuthorizationCode(
	ctx context.Context,
	codeHash [32]byte,
	now time.Time,
	validate func(desktopauth.AuthorizationCode) bool,
) (string, error) {
	if r == nil || r.root == nil {
		return "", fmt.Errorf("desktop authorization database is required")
	}
	if validate == nil {
		return "", fmt.Errorf("desktop authorization validator is required")
	}
	transaction, err := r.root.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin desktop authorization redemption: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	grant, err := readAuthorizationCode(ctx, transaction, codeHash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", desktopauth.ErrInvalidGrant
	}
	if err != nil {
		return "", err
	}
	if !validate(grant) {
		return "", desktopauth.ErrInvalidGrant
	}
	result, err := transaction.ExecContext(ctx, `
UPDATE desktop_authorization_codes
SET consumed_at = ?
WHERE code_hash = ? AND consumed_at IS NULL
`, now.Format(time.RFC3339Nano), codeHash[:])
	if err != nil {
		return "", err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return "", err
	}
	if affected != 1 {
		return "", desktopauth.ErrInvalidGrant
	}
	if err := transaction.Commit(); err != nil {
		return "", fmt.Errorf("commit desktop authorization redemption: %w", err)
	}
	return grant.PrincipalID, nil
}

func readAuthorizationCode(
	ctx context.Context,
	transaction *sql.Tx,
	codeHash [32]byte,
) (desktopauth.AuthorizationCode, error) {
	var (
		grant     desktopauth.AuthorizationCode
		expiresAt string
		createdAt string
		consumed  sql.NullString
	)
	grant.CodeHash = codeHash
	err := transaction.QueryRowContext(ctx, `
SELECT principal_id, client_id, instance_id, profile_id, redirect_uri,
       code_challenge, return_path, expires_at, consumed_at, created_at
FROM desktop_authorization_codes
WHERE code_hash = ?
`, codeHash[:]).Scan(
		&grant.PrincipalID, &grant.ClientID, &grant.InstanceID, &grant.ProfileID,
		&grant.RedirectURI, &grant.CodeChallenge, &grant.ReturnPath, &expiresAt,
		&consumed, &createdAt,
	)
	if err != nil {
		return desktopauth.AuthorizationCode{}, err
	}
	grant.ExpiresAt, err = time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return desktopauth.AuthorizationCode{}, fmt.Errorf(
			"parse desktop authorization expiry: %w",
			err,
		)
	}
	grant.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return desktopauth.AuthorizationCode{}, fmt.Errorf(
			"parse desktop authorization creation time: %w",
			err,
		)
	}
	grant.Consumed = consumed.Valid
	return grant, nil
}
