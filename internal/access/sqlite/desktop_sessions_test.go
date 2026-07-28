package sqlite

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Yacobolo/leapview/internal/access"
	"github.com/Yacobolo/leapview/internal/platform"
)

func TestDesktopSessionMetadataIsTransactionalAndBoundToProfile(t *testing.T) {
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "leapview.db"))
	if err != nil {
		t.Fatalf("open platform store: %v", err)
	}
	defer store.Close()
	repository := NewRepository(store.SQLDB())
	principal, err := repository.SetPlatformRole(t.Context(), access.PlatformRoleInput{
		PrincipalID: "principal_desktop",
		Email:       "desktop@example.com",
		DisplayName: "Desktop User",
		Role:        access.RolePlatformAdmin,
	})
	if err != nil {
		t.Fatalf("create principal: %v", err)
	}
	const (
		instanceID = "instance_0123456789abcdef0123456789abcdef"
		profileID  = "profile_0123456789abcdef0123456789abcdef"
	)
	var token string
	err = repository.RunAuditedMutation(t.Context(), func(txRepository access.Repository) (access.AuditEventInput, error) {
		desktop, ok := txRepository.(access.DesktopSessionRepository)
		if !ok {
			t.Fatal("transaction repository does not implement desktop sessions")
		}
		token, err = desktop.CreateDesktopSession(t.Context(), principal.ID, instanceID, profileID, time.Hour)
		return access.AuditEventInput{
			PrincipalID:  principal.ID,
			Action:       "desktop_session.created",
			TargetType:   "desktop_profile",
			TargetID:     profileID,
			Status:       "success",
			MetadataJSON: "{}",
		}, err
	})
	if err != nil {
		t.Fatalf("create audited desktop session: %v", err)
	}
	binding, err := repository.DesktopSessionForToken(t.Context(), token)
	if err != nil {
		t.Fatalf("read desktop session: %v", err)
	}
	if binding.PrincipalID != principal.ID || binding.InstanceID != instanceID || binding.ProfileID != profileID {
		t.Fatalf("desktop session binding = %#v", binding)
	}
	if err := repository.RevokeDesktopSession(
		t.Context(),
		token,
		instanceID,
		"profile_ffffffffffffffffffffffffffffffff",
	); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("wrong-profile revocation error = %v, want sql.ErrNoRows", err)
	}
	if _, err := repository.DesktopSessionForToken(t.Context(), token); err != nil {
		t.Fatalf("wrong-profile revocation changed the bound session: %v", err)
	}
	if err := repository.RevokeDesktopSession(t.Context(), token, instanceID, profileID); err != nil {
		t.Fatalf("revoke desktop session: %v", err)
	}
	if _, err := repository.PrincipalForToken(t.Context(), token); err == nil {
		t.Fatal("revoked desktop session still authenticates")
	}
	if _, err := repository.DesktopSessionForToken(t.Context(), token); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("revoked desktop session lookup error = %v, want sql.ErrNoRows", err)
	}

	var rolledBackToken string
	mutationErr := errors.New("force audited mutation rollback")
	err = repository.RunAuditedMutation(t.Context(), func(txRepository access.Repository) (access.AuditEventInput, error) {
		desktop := txRepository.(access.DesktopSessionRepository)
		rolledBackToken, err = desktop.CreateDesktopSession(
			t.Context(), principal.ID, instanceID, profileID, time.Hour,
		)
		return access.AuditEventInput{}, mutationErr
	})
	if !errors.Is(err, mutationErr) {
		t.Fatalf("rolled-back mutation error = %v, want %v", err, mutationErr)
	}
	if _, err := repository.DesktopSessionForToken(t.Context(), rolledBackToken); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("rolled-back desktop session lookup error = %v, want sql.ErrNoRows", err)
	}
}
