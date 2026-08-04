package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/flidai/leapview/internal/access"
	accessdb "github.com/flidai/leapview/internal/access/internal/db"
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
	if err := r.q.CreateDesktopSessionBinding(ctx, accessdb.CreateDesktopSessionBindingParams{
		SessionID: id, InstanceID: instanceID, ProfileID: profileID,
		ClientID: "leapview-desktop", AbsoluteExpiresAt: expiresAt,
		CreatedAt: now.Format(time.RFC3339Nano),
	}); err != nil {
		return "", err
	}
	return token, nil
}

func (r *Repository) DesktopSessionForToken(ctx context.Context, token string) (access.DesktopSession, error) {
	fingerprint := secretFingerprint(token)
	row, err := r.q.GetDesktopSessionByTokenFingerprint(
		ctx,
		accessdb.GetDesktopSessionByTokenFingerprintParams{
			TokenFingerprint: fingerprint,
			IdleCutoff: time.Now().UTC().Add(-access.DesktopSessionIdleTimeout).
				Format(time.RFC3339Nano),
		},
	)
	if err != nil {
		return access.DesktopSession{}, err
	}
	if !verifySecret(token, row.TokenVerifier) {
		return access.DesktopSession{}, sql.ErrNoRows
	}
	return access.DesktopSession{
		SessionID: row.ID, PrincipalID: row.PrincipalID,
		InstanceID: row.InstanceID, ProfileID: row.ProfileID,
		ClientID: row.ClientID, ExpiresAt: row.ExpiresAt,
		AbsoluteExpiresAt: row.AbsoluteExpiresAt, CreatedAt: row.CreatedAt,
	}, nil
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
	affected, err := r.q.RevokeActiveSessionByID(ctx, session.SessionID)
	if err != nil {
		return err
	}
	if affected != 1 {
		return sql.ErrNoRows
	}
	return nil
}
