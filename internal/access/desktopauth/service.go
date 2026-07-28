// Package desktopauth implements the first-party public-client authorization
// contract used by LeapView Desktop. It stores only hashes of short-lived
// authorization codes and never creates or returns browser session secrets.
package desktopauth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	DesktopClientID      = "leapview-desktop"
	AuthorizationCodeTTL = 2 * time.Minute

	maxPrincipalIDBytes = 128
	maxReturnPathBytes  = 2048
)

var (
	ErrInvalidRequest = errors.New("invalid desktop authorization request")
	ErrInvalidGrant   = errors.New("invalid desktop authorization grant")

	instanceIDPattern = regexp.MustCompile(`^instance_[0-9a-f]{32}$`)
	profileIDPattern  = regexp.MustCompile(`^profile_[0-9a-f]{32}$`)
	pkceValuePattern  = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
	codePattern       = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
)

type Config struct {
	InstanceID string
	Now        func() time.Time
	Random     io.Reader
}

type Service struct {
	store      Store
	instanceID string
	now        func() time.Time
	random     io.Reader
}

type AuthorizeRequest struct {
	ClientID            string
	ResponseType        string
	CodeChallenge       string
	CodeChallengeMethod string
	State               string
	InstanceID          string
	ProfileID           string
	RedirectURI         string
	ReturnPath          string
}

type AuthorizationResult struct {
	RedirectURL string
}

type RedeemRequest struct {
	ClientID     string
	Code         string
	CodeVerifier string
	InstanceID   string
	ProfileID    string
	RedirectURI  string
}

type AuthorizationCode struct {
	CodeHash      [sha256.Size]byte
	PrincipalID   string
	ClientID      string
	InstanceID    string
	ProfileID     string
	RedirectURI   string
	CodeChallenge string
	ReturnPath    string
	ExpiresAt     time.Time
	Consumed      bool
	CreatedAt     time.Time
}

type Store interface {
	StoreAuthorizationCode(context.Context, AuthorizationCode) error
	ConsumeAuthorizationCode(
		context.Context,
		[sha256.Size]byte,
		time.Time,
		func(AuthorizationCode) bool,
	) (string, error)
}

func New(store Store, config Config) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("desktop authorization store is required")
	}
	if !instanceIDPattern.MatchString(config.InstanceID) {
		return nil, fmt.Errorf("desktop authorization instance id is invalid")
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	random := config.Random
	if random == nil {
		random = rand.Reader
	}
	return &Service{
		store: store, instanceID: config.InstanceID, now: now, random: random,
	}, nil
}

func (s *Service) Issue(ctx context.Context, principalID string, request AuthorizeRequest) (AuthorizationResult, error) {
	redirect, returnPath, err := s.validateAuthorize(principalID, request)
	if err != nil {
		return AuthorizationResult{}, err
	}
	code, err := randomValue(s.random)
	if err != nil {
		return AuthorizationResult{}, fmt.Errorf("generate desktop authorization code: %w", err)
	}
	codeHash := sha256.Sum256([]byte(code))
	now := s.now().UTC()
	expiresAt := now.Add(AuthorizationCodeTTL)
	if err := s.store.StoreAuthorizationCode(ctx, AuthorizationCode{
		CodeHash: codeHash, PrincipalID: principalID, ClientID: request.ClientID,
		InstanceID: request.InstanceID, ProfileID: request.ProfileID,
		RedirectURI: request.RedirectURI, CodeChallenge: request.CodeChallenge,
		ReturnPath: returnPath, ExpiresAt: expiresAt, CreatedAt: now,
	}); err != nil {
		return AuthorizationResult{}, fmt.Errorf("store desktop authorization code: %w", err)
	}
	query := redirect.Query()
	query.Set("code", code)
	query.Set("state", request.State)
	redirect.RawQuery = query.Encode()
	return AuthorizationResult{RedirectURL: redirect.String()}, nil
}

func (s *Service) Redeem(ctx context.Context, request RedeemRequest) (string, error) {
	if err := s.validateRedeem(request); err != nil {
		return "", ErrInvalidGrant
	}
	codeHash := sha256.Sum256([]byte(request.Code))
	verifierDigest := sha256.Sum256([]byte(request.CodeVerifier))
	challenge := base64.RawURLEncoding.EncodeToString(verifierDigest[:])
	now := s.now().UTC()
	principalID, err := s.store.ConsumeAuthorizationCode(
		ctx,
		codeHash,
		now,
		func(grant AuthorizationCode) bool {
			return !grant.Consumed &&
				grant.ExpiresAt.After(now) &&
				constantTimeEqual(grant.ClientID, request.ClientID) &&
				constantTimeEqual(grant.InstanceID, request.InstanceID) &&
				constantTimeEqual(grant.ProfileID, request.ProfileID) &&
				constantTimeEqual(grant.RedirectURI, request.RedirectURI) &&
				constantTimeEqual(grant.CodeChallenge, challenge)
		},
	)
	if err != nil {
		if errors.Is(err, ErrInvalidGrant) {
			return "", ErrInvalidGrant
		}
		return "", fmt.Errorf("consume desktop authorization code: %w", err)
	}
	return principalID, nil
}

func (s *Service) validateAuthorize(principalID string, request AuthorizeRequest) (*url.URL, string, error) {
	if principalID == "" || len(principalID) > maxPrincipalIDBytes || strings.TrimSpace(principalID) != principalID {
		return nil, "", invalidRequest("principal")
	}
	if request.ClientID != DesktopClientID ||
		request.ResponseType != "code" ||
		request.CodeChallengeMethod != "S256" ||
		!pkceValuePattern.MatchString(request.CodeChallenge) ||
		!pkceValuePattern.MatchString(request.State) ||
		request.InstanceID != s.instanceID ||
		!instanceIDPattern.MatchString(request.InstanceID) ||
		!profileIDPattern.MatchString(request.ProfileID) {
		return nil, "", invalidRequest("binding")
	}
	redirect, err := validateLoopbackRedirect(request.RedirectURI)
	if err != nil {
		return nil, "", err
	}
	returnPath := request.ReturnPath
	if returnPath == "" {
		returnPath = "/"
	}
	parsedReturn, err := url.ParseRequestURI(returnPath)
	if err != nil || parsedReturn.IsAbs() || parsedReturn.Host != "" ||
		!strings.HasPrefix(parsedReturn.Path, "/") || strings.HasPrefix(returnPath, "//") ||
		parsedReturn.Fragment != "" || len(returnPath) > maxReturnPathBytes {
		return nil, "", invalidRequest("return path")
	}
	return redirect, returnPath, nil
}

func (s *Service) validateRedeem(request RedeemRequest) error {
	if request.ClientID != DesktopClientID ||
		!codePattern.MatchString(request.Code) ||
		!pkceValuePattern.MatchString(request.CodeVerifier) ||
		request.InstanceID != s.instanceID ||
		!instanceIDPattern.MatchString(request.InstanceID) ||
		!profileIDPattern.MatchString(request.ProfileID) {
		return ErrInvalidGrant
	}
	if _, err := validateLoopbackRedirect(request.RedirectURI); err != nil {
		return ErrInvalidGrant
	}
	return nil
}

func validateLoopbackRedirect(raw string) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" ||
		parsed.User != nil || parsed.Path != "/callback" || parsed.RawPath != "" ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, invalidRequest("redirect URI")
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port < 1024 || port > 65535 ||
		parsed.Host != net.JoinHostPort("127.0.0.1", strconv.Itoa(port)) {
		return nil, invalidRequest("redirect URI")
	}
	return parsed, nil
}

func randomValue(reader io.Reader) (string, error) {
	var value [32]byte
	if _, err := io.ReadFull(reader, value[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}

func constantTimeEqual(left, right string) bool {
	return hmac.Equal([]byte(left), []byte(right))
}

func invalidRequest(field string) error {
	return fmt.Errorf("%w: invalid %s", ErrInvalidRequest, field)
}
