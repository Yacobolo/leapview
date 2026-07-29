package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Yacobolo/leapview/internal/access"
	accessdb "github.com/Yacobolo/leapview/internal/access/internal/db"
)

var (
	desktopInstanceIDPattern = regexp.MustCompile(`^instance_[0-9a-f]{32}$`)
	desktopProfileIDPattern  = regexp.MustCompile(`^profile_[0-9a-f]{32}$`)
)

func (r *Repository) CreateDesktopSession(
	ctx context.Context,
	principalID, instanceID, profileID string,
	ttl time.Duration,
) (string, error) {
	if strings.TrimSpace(principalID) == "" ||
		!desktopInstanceIDPattern.MatchString(instanceID) ||
		!desktopProfileIDPattern.MatchString(profileID) ||
		ttl <= 0 {
		return "", fmt.Errorf("desktop session binding is invalid")
	}
	token, err := newSecret()
	if err != nil {
		return "", err
	}
	fingerprint := secretFingerprint(token)
	verifier, err := newSecretVerifier(token)
	if err != nil {
		return "", err
	}
	id, err := newID("session")
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	expiresAt := now.Add(ttl).Format(time.RFC3339)
	if err := r.q.CreateSession(ctx, accessdb.CreateSessionParams{
		ID:               id,
		PrincipalID:      principalID,
		TokenFingerprint: fingerprint,
		TokenVerifier:    verifier,
		ExpiresAt:        expiresAt,
	}); err != nil {
		return "", err
	}
	if _, err := r.db.ExecContext(ctx, `
INSERT INTO desktop_sessions (
  session_id, instance_id, profile_id, client_id, absolute_expires_at, created_at
) VALUES (?, ?, ?, 'leapview-desktop', ?, ?)
`, id, instanceID, profileID, expiresAt, now.Format(time.RFC3339Nano)); err != nil {
		return "", err
	}
	return token, nil
}

func (r *Repository) DesktopSessionForToken(ctx context.Context, token string) (access.DesktopSession, error) {
	fingerprint := secretFingerprint(token)
	var session access.DesktopSession
	var tokenVerifier string
	err := r.db.QueryRowContext(ctx, `
SELECT s.id, s.principal_id, s.token_verifier, s.expires_at,
       ds.instance_id, ds.profile_id, ds.client_id,
       ds.absolute_expires_at, ds.created_at
FROM sessions s
JOIN desktop_sessions ds ON ds.session_id = s.id
WHERE s.token_fingerprint = ?
  AND s.revoked_at IS NULL
  AND datetime(s.expires_at) > CURRENT_TIMESTAMP
  AND datetime(ds.absolute_expires_at) > CURRENT_TIMESTAMP
`, fingerprint).Scan(
		&session.SessionID, &session.PrincipalID, &tokenVerifier, &session.ExpiresAt,
		&session.InstanceID, &session.ProfileID, &session.ClientID,
		&session.AbsoluteExpiresAt, &session.CreatedAt,
	)
	if err != nil {
		return access.DesktopSession{}, err
	}
	if !verifySecret(token, tokenVerifier) {
		return access.DesktopSession{}, sql.ErrNoRows
	}
	return session, nil
}

func (r *Repository) RevokeDesktopSession(
	ctx context.Context,
	token, instanceID, profileID string,
) error {
	session, err := r.DesktopSessionForToken(ctx, token)
	if err != nil {
		return err
	}
	if session.InstanceID != instanceID || session.ProfileID != profileID {
		return sql.ErrNoRows
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE sessions
SET revoked_at = COALESCE(revoked_at, CURRENT_TIMESTAMP)
WHERE id = ? AND revoked_at IS NULL
`, session.SessionID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return sql.ErrNoRows
	}
	return nil
}
