package offline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"os"
	"strings"
	"time"
)

var ErrInstanceAlreadyInitialized = errors.New("LeapView instance is already initialized")

func (service *Service) Initialize(ctx context.Context, request InitializeRequest, out io.Writer) error {
	if request.Format != "json" {
		return fmt.Errorf("admin initialize supports only --format json")
	}
	email, err := service.initialAdminEmail()
	if err != nil {
		return err
	}
	lock, err := service.acquire(ctx)
	if err != nil {
		return err
	}
	defer lock.Release()

	environment, err := service.resolveEnvironment(ctx)
	if err != nil {
		return err
	}
	initialized, err := service.deps.State.Initialized(ctx)
	if err != nil {
		return err
	}
	if initialized {
		credentials, readErr := service.deps.Recovery.Read()
		if readErr == nil {
			if _, decodeErr := DecodeInitialCredentials(credentials); decodeErr != nil {
				return decodeErr
			}
			return writeAll(out, credentials)
		}
		if os.IsNotExist(readErr) {
			return ErrInstanceAlreadyInitialized
		}
		return readErr
	}
	if err := service.deps.Recovery.Remove(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale initialization credentials: %w", err)
	}

	var encoded []byte
	_, err = service.deps.Initializer.Initialize(ctx, InitializationInput{
		Email: email, Environment: environment, Now: service.deps.Now().UTC(),
	}, func(credentials InitialCredentials) error {
		encoded, err = json.Marshal(credentials)
		if err != nil {
			return err
		}
		encoded = append(encoded, '\n')
		return service.deps.Recovery.Write(encoded)
	})
	if err != nil {
		_ = service.deps.Recovery.Remove()
		return err
	}
	return writeAll(out, encoded)
}

func DecodeInitialCredentials(contents []byte) (InitialCredentials, error) {
	var credentials InitialCredentials
	if err := json.Unmarshal(contents, &credentials); err != nil ||
		credentials.Email == "" ||
		credentials.TemporaryPassword == "" ||
		credentials.PublisherToken == "" ||
		credentials.PublisherTokenExpiresAt == "" {
		return InitialCredentials{}, fmt.Errorf("initialization credential recovery file is invalid")
	}
	return credentials, nil
}

func (service *Service) AcknowledgeInitialCredentials(ctx context.Context) error {
	lock, err := service.acquire(ctx)
	if err != nil {
		return err
	}
	defer lock.Release()
	if _, err := service.resolveEnvironment(ctx); err != nil {
		return err
	}
	initialized, err := service.deps.State.Initialized(ctx)
	if err != nil {
		return err
	}
	if !initialized {
		return fmt.Errorf("LeapView instance has not been initialized")
	}
	if err := service.deps.Recovery.Remove(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("acknowledge initialization credentials: %w", err)
	}
	return nil
}

func (service *Service) StorageCleanup(ctx context.Context, request StorageCleanupRequest, out io.Writer) error {
	release, err := service.acquireIf(ctx, request.Apply)
	if err != nil {
		return err
	}
	defer release.Release()
	environment, err := service.resolveEnvironment(ctx)
	if err != nil {
		return err
	}
	if err := service.deps.Storage.Cleanup(ctx, environment, !request.Apply, out); err != nil {
		return fmt.Errorf("storage cleanup: %w", err)
	}
	return nil
}

func (service *Service) Maintenance(ctx context.Context, request MaintenanceRequest, out io.Writer) error {
	if request.AuditDays < 0 || request.QueryDays < 0 || request.ArchivedAgentDays < 0 || request.AuthStateDays < 0 {
		return fmt.Errorf("retention days must be zero or greater")
	}
	release, err := service.acquireIf(ctx, request.Apply)
	if err != nil {
		return err
	}
	defer release.Release()
	result, err := service.deps.Retention.Prune(ctx, RetentionPolicy{
		AuditEventsMaxAge:             days(request.AuditDays),
		QueryEventsMaxAge:             days(request.QueryDays),
		ArchivedAgentConversationsAge: days(request.ArchivedAgentDays),
		AuthStateMaxAge:               days(request.AuthStateDays),
		DryRun:                        !request.Apply,
	})
	if err != nil {
		return fmt.Errorf("operational maintenance: %w", err)
	}
	mode := "dry-run"
	if request.Apply {
		mode = "apply"
	}
	fmt.Fprintf(out, "mode: %s\n", mode)
	fmt.Fprintf(out, "audit events: %d\n", result.AuditEventsDeleted)
	fmt.Fprintf(out, "query events: %d\n", result.QueryEventsDeleted)
	fmt.Fprintf(out, "archived agent conversations: %d\n", result.ArchivedAgentConversationsDeleted)
	fmt.Fprintf(out, "expired oauth states: %d\n", result.ExpiredOAuthStatesDeleted)
	fmt.Fprintf(out, "stale sessions: %d\n", result.StaleSessionsDeleted)
	fmt.Fprintf(out, "stale api tokens: %d\n", result.StaleAPITokensDeleted)
	fmt.Fprintf(out, "stale service principal secrets: %d\n", result.StaleServicePrincipalSecretsDeleted)
	return nil
}

func (service *Service) resolveEnvironment(ctx context.Context) (string, error) {
	requested := strings.TrimSpace(service.config.Environment)
	bound, err := service.deps.State.Environment(ctx)
	if err == nil {
		if requested != "" && requested != bound {
			return "", fmt.Errorf("LeapView instance is bound to environment %q, not %q", bound, requested)
		}
		return bound, nil
	}
	if !errors.Is(err, ErrStateNotFound) {
		return "", fmt.Errorf("read instance environment: %w", err)
	}
	environment := service.configuredEnvironment()
	if err := service.deps.State.BindEnvironment(ctx, environment); err != nil {
		return "", err
	}
	return environment, nil
}

func (service *Service) configuredEnvironment() string {
	if value := strings.TrimSpace(service.config.Environment); value != "" {
		return value
	}
	if service.config.Production {
		return "prod"
	}
	return "dev"
}

func (service *Service) initialAdminEmail() (string, error) {
	email := strings.TrimSpace(service.config.BootstrapEmail)
	if email == "" {
		if service.config.Production {
			return "", fmt.Errorf("production instance initialization requires LEAPVIEW_BOOTSTRAP_ADMIN_EMAIL")
		}
		email = "admin@localhost"
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address == "" {
		return "", fmt.Errorf("instance initialization requires a valid LEAPVIEW_BOOTSTRAP_ADMIN_EMAIL")
	}
	return parsed.Address, nil
}

func (service *Service) acquire(ctx context.Context) (Lock, error) {
	if service == nil || service.deps.Locker == nil {
		return nil, fmt.Errorf("offline Admin locker is required")
	}
	return service.deps.Locker.Acquire(ctx)
}

type noopLock struct{}

func (noopLock) Release() error { return nil }

func (service *Service) acquireIf(ctx context.Context, required bool) (Lock, error) {
	if !required {
		return noopLock{}, nil
	}
	return service.acquire(ctx)
}

func days(value int) time.Duration {
	if value <= 0 {
		return 0
	}
	return time.Duration(value) * 24 * time.Hour
}

func writeAll(out io.Writer, contents []byte) error {
	written, err := out.Write(contents)
	if err == nil && written != len(contents) {
		return io.ErrShortWrite
	}
	return err
}
