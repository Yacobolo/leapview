package sqlite

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/flidai/leapview/internal/access/desktopauth"
	"github.com/flidai/leapview/internal/platform"
)

func TestDesktopAuthorizationCodeConsumptionIsAtomicInSQLite(t *testing.T) {
	store, err := platform.Open(
		t.Context(),
		filepath.Join(t.TempDir(), "leapview.db"),
	)
	if err != nil {
		t.Fatalf("open platform store: %v", err)
	}
	defer store.Close()
	service, err := desktopauth.New(
		NewRepository(store.SQLDB()),
		desktopauth.Config{
			InstanceID: "instance_0123456789abcdef0123456789abcdef",
		},
	)
	if err != nil {
		t.Fatalf("create desktop authorization service: %v", err)
	}
	const verifier = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFG"
	digest := sha256.Sum256([]byte(verifier))
	issued, err := service.Issue(
		t.Context(),
		"principal_desktop",
		desktopauth.AuthorizeRequest{
			ClientID:            desktopauth.DesktopClientID,
			ResponseType:        "code",
			CodeChallenge:       base64.RawURLEncoding.EncodeToString(digest[:]),
			CodeChallengeMethod: "S256",
			State:               "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq",
			InstanceID:          "instance_0123456789abcdef0123456789abcdef",
			ProfileID:           "profile_0123456789abcdef0123456789abcdef",
			RedirectURI:         "http://127.0.0.1:49152/callback",
			ReturnPath:          "/workspaces",
		},
	)
	if err != nil {
		t.Fatalf("issue authorization code: %v", err)
	}
	callback, err := url.Parse(issued.RedirectURL)
	if err != nil {
		t.Fatalf("parse authorization callback: %v", err)
	}
	request := desktopauth.RedeemRequest{
		ClientID:     desktopauth.DesktopClientID,
		Code:         callback.Query().Get("code"),
		CodeVerifier: verifier,
		InstanceID:   "instance_0123456789abcdef0123456789abcdef",
		ProfileID:    "profile_0123456789abcdef0123456789abcdef",
		RedirectURI:  "http://127.0.0.1:49152/callback",
	}

	start := make(chan struct{})
	outcomes := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, redeemErr := service.Redeem(t.Context(), request)
			outcomes <- redeemErr
		}()
	}
	close(start)

	var successes, replays int
	for range 2 {
		switch redeemErr := <-outcomes; {
		case redeemErr == nil:
			successes++
		case errors.Is(redeemErr, desktopauth.ErrInvalidGrant):
			replays++
		default:
			t.Fatalf("unexpected concurrent redemption error: %v", redeemErr)
		}
	}
	if successes != 1 || replays != 1 {
		t.Fatalf(
			"concurrent SQLite redemptions: successes=%d replays=%d, want 1/1",
			successes,
			replays,
		)
	}
}
