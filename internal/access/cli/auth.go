// Package cli owns command-line adapters for the Access capability.
package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	apigenclient "github.com/Yacobolo/toolbelt/apigen/runtime/client"
	accessgen "github.com/flidai/leapview/internal/access/api/gen"
	"github.com/flidai/leapview/internal/platform/cliapi"
	"github.com/flidai/leapview/internal/platform/securestore"
)

const (
	credentialVersion = 1
	refreshClockSkew  = 30 * time.Second
)

// PublicTransportFactory constructs a transport without resolving credentials.
// Device authorization and refresh exchanges are intentionally public.
type PublicTransportFactory interface {
	PublicTransport(context.Context, string) (apigenclient.Transport, error)
}

type DeviceChallenge struct {
	UserCode                string
	VerificationURI         string
	VerificationURIComplete string
	ExpiresIn               time.Duration
}

type LoginRequest struct {
	Name        string
	Origin      string
	InstanceID  string
	Environment string
	ProjectID   string
	Privileges  []string
	Headless    bool
}

type LoginResult struct {
	SessionID string
	Profile   cliapi.TargetProfile
}

type ResolvedCredential struct {
	Profile     cliapi.TargetProfile
	AccessToken string
	ExpiresAt   time.Time
}

type WorkloadIdentityRequest struct {
	Origin       string
	InstanceID   string
	ProjectID    string
	ClientID     string
	ClientSecret string
	Privileges   []string
	Lifetime     time.Duration
}

type WorkloadIdentityResult struct {
	AccessToken string
	ExpiresAt   time.Time
	SessionID   string
}

type credentialDocument struct {
	Version         int       `json:"version"`
	AccessToken     string    `json:"accessToken"`
	RefreshToken    string    `json:"refreshToken"`
	AccessExpiresAt time.Time `json:"accessExpiresAt"`
	SessionID       string    `json:"sessionId"`
}

// Authenticator implements human CLI device login and credential lifecycle.
// Profiles contain references only; token material stays in the native store.
type Authenticator struct {
	Factory     PublicTransportFactory
	Profiles    *cliapi.ProfileStore
	Secrets     securestore.Store
	OpenBrowser func(string) error
	Now         func() time.Time
	Wait        func(context.Context, time.Duration) error
}

func (auth Authenticator) Login(ctx context.Context, request LoginRequest, notify func(DeviceChallenge)) (LoginResult, error) {
	if err := auth.validate(); err != nil {
		return LoginResult{}, err
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Origin = strings.TrimRight(strings.TrimSpace(request.Origin), "/")
	request.InstanceID = strings.TrimSpace(request.InstanceID)
	request.ProjectID = strings.TrimSpace(request.ProjectID)
	if request.Name == "" || request.Origin == "" || request.InstanceID == "" || request.ProjectID == "" {
		return LoginResult{}, fmt.Errorf("login target name, origin, instance identity, and project are required")
	}
	if len(request.Privileges) == 0 {
		return LoginResult{}, fmt.Errorf("login requires at least one authoring privilege")
	}
	if existing, err := auth.Profiles.Get(request.Name); err == nil {
		if existing.InstanceID != request.InstanceID || existing.Origin != request.Origin || existing.ProjectID != request.ProjectID {
			return LoginResult{}, fmt.Errorf("target profile %q already identifies another instance, origin, or project; log out before replacing it", request.Name)
		}
	} else if !errors.Is(err, cliapi.ErrProfileNotFound) {
		return LoginResult{}, err
	}
	transport, err := auth.Factory.PublicTransport(ctx, request.Origin)
	if err != nil {
		return LoginResult{}, fmt.Errorf("create Access API transport: %w", err)
	}
	client := accessgen.NewGenClient(transport)
	start, err := client.BeginDeviceAuthorization(ctx, accessgen.GenBeginDeviceAuthorizationClientRequest{
		Body: accessgen.GenSchemaDeviceAuthorizationStartRequest{
			Scope: accessgen.GenSchemaAuthoringScopeRequest{
				ProjectId: request.ProjectID, Privileges: append([]string(nil), request.Privileges...),
			},
		},
	})
	if err != nil {
		return LoginResult{}, fmt.Errorf("begin device authorization: %w", err)
	}
	challenge := DeviceChallenge{
		UserCode: start.Body.UserCode, VerificationURI: start.Body.VerificationUri,
		VerificationURIComplete: start.Body.VerificationUriComplete,
		ExpiresIn:               time.Duration(start.Body.ExpiresIn) * time.Second,
	}
	if notify != nil {
		notify(challenge)
	}
	if !request.Headless && auth.OpenBrowser != nil {
		if err := auth.OpenBrowser(start.Body.VerificationUriComplete); err != nil {
			return LoginResult{}, fmt.Errorf("open device authorization in browser: %w", err)
		}
	}
	interval := time.Duration(start.Body.Interval) * time.Second
	if interval <= 0 {
		interval = time.Second
	}
	expiresAt := auth.now().Add(challenge.ExpiresIn)
	var tokens accessgen.GenSchemaAuthoringTokenResponse
	for {
		if !auth.now().Before(expiresAt) {
			return LoginResult{}, fmt.Errorf("device authorization expired")
		}
		if err := auth.wait(ctx, interval); err != nil {
			return LoginResult{}, err
		}
		exchange, exchangeErr := client.ExchangeDeviceAuthorization(ctx, accessgen.GenExchangeDeviceAuthorizationClientRequest{
			Body: accessgen.GenSchemaDeviceAuthorizationTokenRequest{DeviceCode: start.Body.DeviceCode},
		})
		if exchangeErr == nil {
			tokens = exchange.Body
			break
		}
		switch exchange.StatusCode {
		case http.StatusConflict:
			continue
		case http.StatusTooManyRequests:
			interval += time.Second
			continue
		default:
			return LoginResult{}, fmt.Errorf("exchange device authorization: %w", exchangeErr)
		}
	}
	if tokens.RefreshToken == nil || strings.TrimSpace(*tokens.RefreshToken) == "" {
		return LoginResult{}, fmt.Errorf("device authorization returned no refresh credential")
	}
	if tokens.Session.TargetId != request.InstanceID || tokens.Session.ProjectId != request.ProjectID {
		return LoginResult{}, fmt.Errorf("device authorization returned credentials for an unexpected target or project")
	}
	account := credentialAccount(request.InstanceID, request.ProjectID)
	credential := credentialDocument{
		Version: credentialVersion, AccessToken: tokens.AccessToken, RefreshToken: *tokens.RefreshToken,
		AccessExpiresAt: auth.now().Add(time.Duration(tokens.ExpiresIn) * time.Second), SessionID: tokens.Session.Id,
	}
	if err := auth.storeCredential(ctx, account, credential); err != nil {
		return LoginResult{}, err
	}
	profile := cliapi.TargetProfile{
		Origin: request.Origin, InstanceID: request.InstanceID, Environment: request.Environment,
		ProjectID: request.ProjectID, CredentialAccount: account,
	}
	if err := auth.Profiles.Put(request.Name, profile); err != nil {
		_ = auth.Secrets.Delete(ctx, account)
		return LoginResult{}, err
	}
	return LoginResult{SessionID: tokens.Session.Id, Profile: profile}, nil
}

// Resolve returns a usable access credential, rotating it before expiry to
// absorb ordinary client/server clock skew and long-running publish steps.
func (auth Authenticator) Resolve(ctx context.Context, name string) (ResolvedCredential, error) {
	if err := auth.validate(); err != nil {
		return ResolvedCredential{}, err
	}
	profile, err := auth.Profiles.Get(strings.TrimSpace(name))
	if err != nil {
		return ResolvedCredential{}, err
	}
	credential, err := auth.loadCredential(ctx, profile.CredentialAccount)
	if err != nil {
		return ResolvedCredential{}, err
	}
	if auth.now().Add(refreshClockSkew).Before(credential.AccessExpiresAt) {
		return ResolvedCredential{Profile: profile, AccessToken: credential.AccessToken, ExpiresAt: credential.AccessExpiresAt}, nil
	}
	transport, err := auth.Factory.PublicTransport(ctx, profile.Origin)
	if err != nil {
		return ResolvedCredential{}, err
	}
	response, refreshErr := accessgen.NewGenClient(transport).RefreshAuthoringToken(ctx, accessgen.GenRefreshAuthoringTokenClientRequest{
		Body: accessgen.GenSchemaAuthoringRefreshRequest{RefreshToken: credential.RefreshToken},
	})
	if refreshErr != nil {
		if invalidRefreshStatus(response.StatusCode) {
			cleanupErr := auth.purgeLocal(ctx, name, profile.CredentialAccount)
			if cleanupErr != nil {
				return ResolvedCredential{}, errors.Join(refreshErr, cleanupErr)
			}
		}
		return ResolvedCredential{}, fmt.Errorf("refresh CLI credential: %w", refreshErr)
	}
	if response.Body.RefreshToken == nil || *response.Body.RefreshToken == "" {
		return ResolvedCredential{}, fmt.Errorf("refresh response omitted the rotated refresh credential")
	}
	if response.Body.Session.TargetId != profile.InstanceID || response.Body.Session.ProjectId != profile.ProjectID {
		return ResolvedCredential{}, fmt.Errorf("refresh response changed target or project scope")
	}
	credential = credentialDocument{
		Version: credentialVersion, AccessToken: response.Body.AccessToken, RefreshToken: *response.Body.RefreshToken,
		AccessExpiresAt: auth.now().Add(time.Duration(response.Body.ExpiresIn) * time.Second),
		SessionID:       response.Body.Session.Id,
	}
	if err := auth.storeCredential(ctx, profile.CredentialAccount, credential); err != nil {
		return ResolvedCredential{}, err
	}
	return ResolvedCredential{Profile: profile, AccessToken: credential.AccessToken, ExpiresAt: credential.AccessExpiresAt}, nil
}

// ResolveOrigin selects the exact profile whose native credential matches the
// access token rejected during a long-running operation, then rotates it.
func (auth Authenticator) ResolveOrigin(ctx context.Context, origin, accessToken string) (ResolvedCredential, error) {
	if err := auth.validate(); err != nil {
		return ResolvedCredential{}, err
	}
	profiles, err := auth.Profiles.ProfilesByOrigin(origin)
	if err != nil {
		return ResolvedCredential{}, err
	}
	for _, candidate := range profiles {
		credential, err := auth.loadCredential(ctx, candidate.Profile.CredentialAccount)
		if errors.Is(err, securestore.ErrNotFound) {
			continue
		}
		if err != nil {
			return ResolvedCredential{}, err
		}
		if credential.AccessToken == accessToken {
			return auth.Resolve(ctx, candidate.Name)
		}
	}
	return ResolvedCredential{}, cliapi.ErrProfileNotFound
}

func (auth Authenticator) Logout(ctx context.Context, name string) error {
	if err := auth.validate(); err != nil {
		return err
	}
	profile, err := auth.Profiles.Get(strings.TrimSpace(name))
	if err != nil {
		return err
	}
	credential, credentialErr := auth.loadCredential(ctx, profile.CredentialAccount)
	var revokeErr error
	if credentialErr == nil {
		if transport, err := auth.Factory.PublicTransport(ctx, profile.Origin); err != nil {
			revokeErr = err
		} else {
			_, revokeErr = accessgen.NewGenClient(transport).RevokeAuthoringToken(ctx, accessgen.GenRevokeAuthoringTokenClientRequest{
				Body: accessgen.GenSchemaAuthoringRevokeRequest{AccessToken: credential.AccessToken},
			})
		}
	}
	cleanupErr := auth.purgeLocal(ctx, name, profile.CredentialAccount)
	if errors.Is(credentialErr, securestore.ErrNotFound) {
		credentialErr = nil
	}
	return errors.Join(revokeErr, credentialErr, cleanupErr)
}

// ExchangeWorkloadIdentity creates a non-refreshable, exact-scope credential
// for an ephemeral CI workload. The service-principal secret and returned
// access token are never persisted by this adapter.
func ExchangeWorkloadIdentity(
	ctx context.Context,
	factory PublicTransportFactory,
	request WorkloadIdentityRequest,
	now func() time.Time,
) (WorkloadIdentityResult, error) {
	if factory == nil {
		return WorkloadIdentityResult{}, fmt.Errorf("Access public transport factory is required")
	}
	if strings.TrimSpace(request.Origin) == "" || strings.TrimSpace(request.InstanceID) == "" ||
		strings.TrimSpace(request.ProjectID) == "" || strings.TrimSpace(request.ClientID) == "" ||
		strings.TrimSpace(request.ClientSecret) == "" || len(request.Privileges) == 0 ||
		request.Lifetime <= 0 {
		return WorkloadIdentityResult{}, fmt.Errorf("workload target, instance, project, client credentials, privileges, and lifetime are required")
	}
	transport, err := factory.PublicTransport(ctx, request.Origin)
	if err != nil {
		return WorkloadIdentityResult{}, err
	}
	response, err := accessgen.NewGenClient(transport).ExchangeWorkloadIdentity(ctx, accessgen.GenExchangeWorkloadIdentityClientRequest{
		Body: accessgen.GenSchemaWorkloadIdentityTokenRequest{
			ClientId: request.ClientID, ClientSecret: request.ClientSecret,
			Scope: accessgen.GenSchemaAuthoringScopeRequest{
				ProjectId: request.ProjectID, Privileges: append([]string(nil), request.Privileges...),
			},
			LifetimeSeconds: int64(request.Lifetime / time.Second),
		},
	})
	if err != nil {
		return WorkloadIdentityResult{}, fmt.Errorf("exchange workload identity: %w", err)
	}
	if response.Body.RefreshToken != nil {
		return WorkloadIdentityResult{}, fmt.Errorf("workload identity unexpectedly returned a refresh credential")
	}
	if response.Body.Session.Kind != "workload" || response.Body.Session.TargetId != request.InstanceID ||
		response.Body.Session.ProjectId != request.ProjectID || response.Body.ExpiresIn <= 0 ||
		response.Body.ExpiresIn > int64(request.Lifetime/time.Second) {
		return WorkloadIdentityResult{}, fmt.Errorf("workload identity response changed the requested scope or lifetime")
	}
	clock := time.Now
	if now != nil {
		clock = now
	}
	return WorkloadIdentityResult{
		AccessToken: response.Body.AccessToken,
		ExpiresAt:   clock().UTC().Add(time.Duration(response.Body.ExpiresIn) * time.Second),
		SessionID:   response.Body.Session.Id,
	}, nil
}

func (auth Authenticator) validate() error {
	switch {
	case auth.Factory == nil:
		return fmt.Errorf("Access public transport factory is required")
	case auth.Profiles == nil:
		return fmt.Errorf("target profile store is required")
	case auth.Secrets == nil:
		return fmt.Errorf("native credential store is required")
	default:
		return nil
	}
}

func (auth Authenticator) now() time.Time {
	if auth.Now != nil {
		return auth.Now().UTC()
	}
	return time.Now().UTC()
}

func (auth Authenticator) wait(ctx context.Context, duration time.Duration) error {
	if auth.Wait != nil {
		return auth.Wait(ctx, duration)
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (auth Authenticator) storeCredential(ctx context.Context, account string, credential credentialDocument) error {
	content, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("encode native credential: %w", err)
	}
	if err := auth.Secrets.Set(ctx, account, string(content)); err != nil {
		return fmt.Errorf("store native credential: %w", err)
	}
	return nil
}

func (auth Authenticator) loadCredential(ctx context.Context, account string) (credentialDocument, error) {
	content, err := auth.Secrets.Get(ctx, account)
	if err != nil {
		return credentialDocument{}, err
	}
	var credential credentialDocument
	if err := json.Unmarshal([]byte(content), &credential); err != nil {
		return credentialDocument{}, fmt.Errorf("decode native credential: %w", err)
	}
	if credential.Version != credentialVersion || credential.AccessToken == "" || credential.RefreshToken == "" ||
		credential.AccessExpiresAt.IsZero() || credential.SessionID == "" {
		return credentialDocument{}, fmt.Errorf("native credential is incomplete or uses an unsupported version")
	}
	return credential, nil
}

func (auth Authenticator) purgeLocal(ctx context.Context, name, account string) error {
	secretErr := auth.Secrets.Delete(ctx, account)
	if errors.Is(secretErr, securestore.ErrNotFound) {
		secretErr = nil
	}
	profileErr := auth.Profiles.Delete(name)
	if errors.Is(profileErr, cliapi.ErrProfileNotFound) {
		profileErr = nil
	}
	return errors.Join(secretErr, profileErr)
}

func credentialAccount(instanceID, projectID string) string {
	digest := sha256.Sum256([]byte(instanceID + "\x00" + projectID))
	return "target/" + hex.EncodeToString(digest[:])
}

func invalidRefreshStatus(status int) bool {
	switch status {
	case http.StatusBadRequest, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity:
		return true
	default:
		return false
	}
}
