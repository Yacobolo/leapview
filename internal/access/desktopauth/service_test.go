package desktopauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"sync"
	"testing"
	"time"
)

const (
	testInstanceID = "instance_0123456789abcdef0123456789abcdef"
	testProfileID  = "profile_0123456789abcdef0123456789abcdef"
	testVerifier   = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFG"
)

func TestIssueAndRedeemAuthorizationCode(t *testing.T) {
	service := newTestService(t)
	request := validAuthorizeRequest()

	result, err := service.Issue(t.Context(), "principal_1", request)
	if err != nil {
		t.Fatalf("issue desktop authorization code: %v", err)
	}
	callback, err := url.Parse(result.RedirectURL)
	if err != nil {
		t.Fatalf("parse callback URL: %v", err)
	}
	if callback.Scheme != "http" || callback.Host != "127.0.0.1:49152" || callback.Path != "/callback" {
		t.Fatalf("callback URL = %q, want exact loopback redirect", result.RedirectURL)
	}
	if callback.Query().Get("state") != request.State {
		t.Fatalf("callback state = %q, want %q", callback.Query().Get("state"), request.State)
	}
	code := callback.Query().Get("code")
	if code == "" {
		t.Fatal("callback authorization code is empty")
	}

	principalID, err := service.Redeem(t.Context(), RedeemRequest{
		ClientID:     DesktopClientID,
		Code:         code,
		CodeVerifier: testVerifier,
		InstanceID:   testInstanceID,
		ProfileID:    testProfileID,
		RedirectURI:  "http://127.0.0.1:49152/callback",
	})
	if err != nil {
		t.Fatalf("redeem desktop authorization code: %v", err)
	}
	if principalID != "principal_1" {
		t.Fatalf("principal ID = %q, want principal_1", principalID)
	}
}

func TestAuthorizationCodeIsSingleUseUnderConcurrency(t *testing.T) {
	service := newTestService(t)
	result, err := service.Issue(t.Context(), "principal_1", validAuthorizeRequest())
	if err != nil {
		t.Fatalf("issue desktop authorization code: %v", err)
	}
	code := callbackCode(t, result.RedirectURL)
	request := validRedeemRequest(code)

	type outcome struct {
		principalID string
		err         error
	}
	start := make(chan struct{})
	outcomes := make(chan outcome, 2)
	for range 2 {
		go func() {
			<-start
			principalID, redeemErr := service.Redeem(t.Context(), request)
			outcomes <- outcome{principalID: principalID, err: redeemErr}
		}()
	}
	close(start)

	var successes, replays int
	for range 2 {
		result := <-outcomes
		switch {
		case result.err == nil && result.principalID == "principal_1":
			successes++
		case errors.Is(result.err, ErrInvalidGrant):
			replays++
		default:
			t.Fatalf("unexpected concurrent redemption outcome: principal=%q err=%v", result.principalID, result.err)
		}
	}
	if successes != 1 || replays != 1 {
		t.Fatalf("concurrent redemptions: successes=%d replays=%d, want 1/1", successes, replays)
	}
}

func TestRedeemFailsClosedForWrongBindingAndExpiry(t *testing.T) {
	tests := map[string]func(*RedeemRequest){
		"wrong verifier": func(request *RedeemRequest) { request.CodeVerifier = testVerifier + "x" },
		"wrong instance": func(request *RedeemRequest) { request.InstanceID = "instance_ffffffffffffffffffffffffffffffff" },
		"wrong profile":  func(request *RedeemRequest) { request.ProfileID = "profile_ffffffffffffffffffffffffffffffff" },
		"wrong redirect": func(request *RedeemRequest) { request.RedirectURI = "http://127.0.0.1:49153/callback" },
		"wrong client":   func(request *RedeemRequest) { request.ClientID = "other-client" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			service := newTestService(t)
			result, err := service.Issue(t.Context(), "principal_1", validAuthorizeRequest())
			if err != nil {
				t.Fatalf("issue desktop authorization code: %v", err)
			}
			request := validRedeemRequest(callbackCode(t, result.RedirectURL))
			mutate(&request)
			if _, err := service.Redeem(t.Context(), request); !errors.Is(err, ErrInvalidGrant) {
				t.Fatalf("redeem error = %v, want ErrInvalidGrant", err)
			}
		})
	}

	t.Run("expired", func(t *testing.T) {
		now := time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC)
		service := newTestServiceAt(t, func() time.Time { return now })
		result, err := service.Issue(t.Context(), "principal_1", validAuthorizeRequest())
		if err != nil {
			t.Fatalf("issue desktop authorization code: %v", err)
		}
		now = now.Add(AuthorizationCodeTTL + time.Second)
		if _, err := service.Redeem(t.Context(), validRedeemRequest(callbackCode(t, result.RedirectURL))); !errors.Is(err, ErrInvalidGrant) {
			t.Fatalf("expired redeem error = %v, want ErrInvalidGrant", err)
		}
	})
}

func TestIssueRejectsUnsafeAuthorizationRequests(t *testing.T) {
	tests := map[string]func(*AuthorizeRequest){
		"wrong client":        func(request *AuthorizeRequest) { request.ClientID = "other-client" },
		"wrong response type": func(request *AuthorizeRequest) { request.ResponseType = "token" },
		"wrong challenge mode": func(request *AuthorizeRequest) {
			request.CodeChallengeMethod = "plain"
		},
		"invalid challenge": func(request *AuthorizeRequest) { request.CodeChallenge = "short" },
		"missing state":     func(request *AuthorizeRequest) { request.State = "" },
		"remote callback":   func(request *AuthorizeRequest) { request.RedirectURI = "https://attacker.example/callback" },
		"localhost callback": func(request *AuthorizeRequest) {
			request.RedirectURI = "http://localhost:49152/callback"
		},
		"callback query": func(request *AuthorizeRequest) {
			request.RedirectURI = "http://127.0.0.1:49152/callback?leak=true"
		},
		"privileged callback port": func(request *AuthorizeRequest) {
			request.RedirectURI = "http://127.0.0.1:80/callback"
		},
		"wrong callback path": func(request *AuthorizeRequest) {
			request.RedirectURI = "http://127.0.0.1:49152/other"
		},
		"wrong instance": func(request *AuthorizeRequest) {
			request.InstanceID = "instance_ffffffffffffffffffffffffffffffff"
		},
		"invalid profile": func(request *AuthorizeRequest) { request.ProfileID = "profile_acme" },
		"absolute return path": func(request *AuthorizeRequest) {
			request.ReturnPath = "https://attacker.example"
		},
		"scheme-relative return path": func(request *AuthorizeRequest) { request.ReturnPath = "//attacker.example" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			service := newTestService(t)
			request := validAuthorizeRequest()
			mutate(&request)
			if _, err := service.Issue(t.Context(), "principal_1", request); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("issue error = %v, want ErrInvalidRequest", err)
			}
		})
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	return newTestServiceAt(t, time.Now)
}

func newTestServiceAt(t *testing.T, now func() time.Time) *Service {
	t.Helper()
	service, err := New(newMemoryAuthorizationStore(), Config{
		InstanceID: testInstanceID,
		Now:        now,
	})
	if err != nil {
		t.Fatalf("new desktop auth service: %v", err)
	}
	return service
}

type memoryAuthorizationStore struct {
	mu     sync.Mutex
	grants map[[sha256.Size]byte]AuthorizationCode
}

func newMemoryAuthorizationStore() *memoryAuthorizationStore {
	return &memoryAuthorizationStore{
		grants: make(map[[sha256.Size]byte]AuthorizationCode),
	}
}

func (s *memoryAuthorizationStore) StoreAuthorizationCode(
	_ context.Context,
	code AuthorizationCode,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for hash, grant := range s.grants {
		if !grant.ExpiresAt.After(code.CreatedAt) || grant.Consumed {
			delete(s.grants, hash)
		}
	}
	s.grants[code.CodeHash] = code
	return nil
}

func (s *memoryAuthorizationStore) ConsumeAuthorizationCode(
	_ context.Context,
	codeHash [sha256.Size]byte,
	_ time.Time,
	validate func(AuthorizationCode) bool,
) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	grant, ok := s.grants[codeHash]
	if !ok || !validate(grant) {
		return "", ErrInvalidGrant
	}
	grant.Consumed = true
	s.grants[codeHash] = grant
	return grant.PrincipalID, nil
}

func validAuthorizeRequest() AuthorizeRequest {
	digest := sha256.Sum256([]byte(testVerifier))
	return AuthorizeRequest{
		ClientID:            DesktopClientID,
		ResponseType:        "code",
		CodeChallenge:       base64.RawURLEncoding.EncodeToString(digest[:]),
		CodeChallengeMethod: "S256",
		State:               "0123456789abcdefghijklmnopqrstuvwxyzABCDEFG",
		InstanceID:          testInstanceID,
		ProfileID:           testProfileID,
		RedirectURI:         "http://127.0.0.1:49152/callback",
		ReturnPath:          "/workspaces",
	}
}

func validRedeemRequest(code string) RedeemRequest {
	return RedeemRequest{
		ClientID:     DesktopClientID,
		Code:         code,
		CodeVerifier: testVerifier,
		InstanceID:   testInstanceID,
		ProfileID:    testProfileID,
		RedirectURI:  "http://127.0.0.1:49152/callback",
	}
}

func callbackCode(t *testing.T, rawURL string) string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse callback URL: %v", err)
	}
	code := parsed.Query().Get("code")
	if code == "" {
		t.Fatal("callback code is empty")
	}
	return code
}
