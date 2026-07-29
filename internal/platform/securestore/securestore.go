// Package securestore provides fail-closed access to operating-system native
// credential storage. It deliberately has no plaintext file fallback.
package securestore

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

var (
	// ErrNotFound reports that an account has no credential in the native store.
	ErrNotFound        = errors.New("secure credential not found")
	errBackendNotFound = errors.New("native credential not found")
)

// Store is the narrow secret-storage port used by CLI capability adapters.
type Store interface {
	Set(context.Context, string, string) error
	Get(context.Context, string) (string, error)
	Delete(context.Context, string) error
}

type backend interface {
	Set(service, account, secret string) error
	Get(service, account string) (string, error)
	Delete(service, account string) error
}

type keyringBackend struct{}

func (keyringBackend) Set(service, account, secret string) error {
	return keyring.Set(service, account, secret)
}

func (keyringBackend) Get(service, account string) (string, error) {
	value, err := keyring.Get(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", errBackendNotFound
	}
	return value, err
}

func (keyringBackend) Delete(service, account string) error {
	err := keyring.Delete(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return errBackendNotFound
	}
	return err
}

// Native stores credentials under an application-specific OS keychain service.
type Native struct {
	service string
	backend backend
}

// NewNative constructs an OS-native credential store. The service name is a
// security boundary: the CLI and Desktop application must use different names.
func NewNative(service string) (*Native, error) {
	return newNative(service, keyringBackend{})
}

func newNative(service string, backend backend) (*Native, error) {
	service = strings.TrimSpace(service)
	if service == "" {
		return nil, fmt.Errorf("secure-store service is required")
	}
	if backend == nil {
		return nil, fmt.Errorf("secure-store backend is required")
	}
	return &Native{service: service, backend: backend}, nil
}

func (store *Native) Set(ctx context.Context, account, secret string) error {
	if err := validateOperation(ctx, account); err != nil {
		return err
	}
	if secret == "" {
		return fmt.Errorf("secure-store secret is required")
	}
	if err := store.backend.Set(store.service, account, secret); err != nil {
		return fmt.Errorf("store native credential: %w", err)
	}
	return nil
}

func (store *Native) Get(ctx context.Context, account string) (string, error) {
	if err := validateOperation(ctx, account); err != nil {
		return "", err
	}
	secret, err := store.backend.Get(store.service, account)
	if errors.Is(err, errBackendNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read native credential: %w", err)
	}
	return secret, nil
}

func (store *Native) Delete(ctx context.Context, account string) error {
	if err := validateOperation(ctx, account); err != nil {
		return err
	}
	err := store.backend.Delete(store.service, account)
	if errors.Is(err, errBackendNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("delete native credential: %w", err)
	}
	return nil
}

func validateOperation(ctx context.Context, account string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(account) == "" {
		return fmt.Errorf("secure-store account is required")
	}
	return nil
}
