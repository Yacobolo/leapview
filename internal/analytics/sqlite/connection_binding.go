package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/flidai/leapview/internal/analytics/connectionbinding"
	analyticsdb "github.com/flidai/leapview/internal/analytics/internal/db"
)

type ConnectionBindingRepository struct {
	q *analyticsdb.Queries
}

var _ connectionbinding.Repository = (*ConnectionBindingRepository)(nil)

func NewConnectionBindingRepository(database *sql.DB) *ConnectionBindingRepository {
	return &ConnectionBindingRepository{q: analyticsdb.New(database)}
}

func (repository *ConnectionBindingRepository) Create(ctx context.Context, binding connectionbinding.TargetBinding) error {
	if repository == nil || repository.q == nil || binding.Revision != 1 {
		return fmt.Errorf("%w: repository and new binding are required", connectionbinding.ErrInvalidBinding)
	}
	if err := binding.Validate(); err != nil {
		return err
	}
	endpoint, err := json.Marshal(binding.Endpoint)
	if err != nil {
		return fmt.Errorf("encode non-secret endpoint: %w", err)
	}
	err = repository.q.CreateTargetConnectionBinding(ctx, analyticsdb.CreateTargetConnectionBindingParams{
		ID: binding.ID, TargetID: binding.TargetID, LogicalConnectionID: binding.LogicalConnectionID.String(),
		ConnectorKind: binding.ConnectorKind, AuthenticationMode: string(binding.AuthenticationMode),
		WorkspaceID: binding.Scope.WorkspaceID, Environment: binding.Scope.Environment, EndpointJson: string(endpoint),
		CredentialProjectID: binding.CredentialReference.ProjectID, CredentialEnvironment: binding.CredentialReference.Environment,
		CredentialSecretPath: binding.CredentialReference.SecretPath, CredentialSecretKey: binding.CredentialReference.SecretKey,
		Enabled: boolInt(binding.Enabled), ValidatedVersion: binding.ValidatedVersion, Health: string(binding.Health),
		HealthReason: binding.HealthReason, LastValidatedAt: nullableTime(binding.LastValidatedAt),
		CreatedAt: sqliteTime(binding.CreatedAt), UpdatedAt: sqliteTime(binding.UpdatedAt), Revision: binding.Revision,
	})
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "constraint") {
		return fmt.Errorf("%w: target connection scope already has a binding", connectionbinding.ErrIncompatibleBinding)
	}
	return err
}

func (repository *ConnectionBindingRepository) Binding(
	ctx context.Context,
	scope connectionbinding.BindingScope,
	targetID string,
	logicalID connectionbinding.LogicalConnectionID,
) (connectionbinding.TargetBinding, error) {
	if repository == nil || repository.q == nil {
		return connectionbinding.TargetBinding{}, connectionbinding.ErrBindingNotFound
	}
	row, err := repository.q.GetTargetConnectionBinding(ctx, analyticsdb.GetTargetConnectionBindingParams{
		TargetID: strings.TrimSpace(targetID), WorkspaceID: strings.TrimSpace(scope.WorkspaceID),
		Environment: strings.TrimSpace(scope.Environment), LogicalConnectionID: logicalID.String(),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return connectionbinding.TargetBinding{}, connectionbinding.ErrBindingNotFound
	}
	if err != nil {
		return connectionbinding.TargetBinding{}, err
	}
	return bindingFromDB(row)
}

func (repository *ConnectionBindingRepository) Save(
	ctx context.Context,
	binding connectionbinding.TargetBinding,
	expectedRevision int64,
) (connectionbinding.TargetBinding, error) {
	if repository == nil || repository.q == nil || expectedRevision <= 0 || binding.Revision != expectedRevision+1 {
		return connectionbinding.TargetBinding{}, fmt.Errorf("%w: invalid binding revision", connectionbinding.ErrIncompatibleBinding)
	}
	if err := binding.Validate(); err != nil {
		return connectionbinding.TargetBinding{}, err
	}
	endpoint, err := json.Marshal(binding.Endpoint)
	if err != nil {
		return connectionbinding.TargetBinding{}, fmt.Errorf("encode non-secret endpoint: %w", err)
	}
	count, err := repository.q.UpdateTargetConnectionBinding(ctx, analyticsdb.UpdateTargetConnectionBindingParams{
		EndpointJson: string(endpoint), CredentialProjectID: binding.CredentialReference.ProjectID,
		CredentialEnvironment: binding.CredentialReference.Environment,
		CredentialSecretPath:  binding.CredentialReference.SecretPath, CredentialSecretKey: binding.CredentialReference.SecretKey,
		Enabled: boolInt(binding.Enabled), ValidatedVersion: binding.ValidatedVersion, Health: string(binding.Health),
		HealthReason: binding.HealthReason, LastValidatedAt: nullableTime(binding.LastValidatedAt),
		UpdatedAt: sqliteTime(binding.UpdatedAt), Revision: binding.Revision, ID: binding.ID, Revision_2: expectedRevision,
		TargetID: binding.TargetID, LogicalConnectionID: binding.LogicalConnectionID.String(),
		ConnectorKind: binding.ConnectorKind, AuthenticationMode: string(binding.AuthenticationMode),
		WorkspaceID: binding.Scope.WorkspaceID, Environment: binding.Scope.Environment,
	})
	if err != nil {
		return connectionbinding.TargetBinding{}, err
	}
	if count != 1 {
		return connectionbinding.TargetBinding{}, connectionbinding.ErrIncompatibleBinding
	}
	return binding, nil
}

func bindingFromDB(row analyticsdb.TargetConnectionBinding) (connectionbinding.TargetBinding, error) {
	logicalID, err := connectionbinding.ParseLogicalConnectionID(row.LogicalConnectionID)
	if err != nil {
		return connectionbinding.TargetBinding{}, err
	}
	var endpoint connectionbinding.EndpointConfig
	if err := json.Unmarshal([]byte(row.EndpointJson), &endpoint); err != nil {
		return connectionbinding.TargetBinding{}, fmt.Errorf("decode non-secret endpoint: %w", err)
	}
	createdAt, err := parseTime(row.CreatedAt)
	if err != nil {
		return connectionbinding.TargetBinding{}, fmt.Errorf("parse binding creation: %w", err)
	}
	updatedAt, err := parseTime(row.UpdatedAt)
	if err != nil {
		return connectionbinding.TargetBinding{}, fmt.Errorf("parse binding update: %w", err)
	}
	lastValidatedAt, err := parseNullableTime(row.LastValidatedAt)
	if err != nil {
		return connectionbinding.TargetBinding{}, fmt.Errorf("parse binding validation: %w", err)
	}
	binding := connectionbinding.TargetBinding{
		ID: row.ID, TargetID: row.TargetID, LogicalConnectionID: logicalID, ConnectorKind: row.ConnectorKind,
		AuthenticationMode: connectionbinding.AuthenticationMode(row.AuthenticationMode),
		Scope:              connectionbinding.BindingScope{WorkspaceID: row.WorkspaceID, Environment: row.Environment},
		Endpoint:           endpoint, CredentialReference: connectionbinding.CredentialReference{
			ProjectID: row.CredentialProjectID, Environment: row.CredentialEnvironment,
			SecretPath: row.CredentialSecretPath, SecretKey: row.CredentialSecretKey,
		},
		Enabled: row.Enabled == 1, ValidatedVersion: row.ValidatedVersion,
		Health: connectionbinding.BindingHealth(row.Health), HealthReason: row.HealthReason,
		LastValidatedAt: lastValidatedAt, CreatedAt: createdAt, UpdatedAt: updatedAt, Revision: row.Revision,
	}
	if err := binding.Validate(); err != nil {
		return connectionbinding.TargetBinding{}, fmt.Errorf("validate persisted binding: %w", err)
	}
	return binding, nil
}

func boolInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func sqliteTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func nullableTime(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: sqliteTime(value), Valid: true}
}

func parseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

func parseNullableTime(value sql.NullString) (time.Time, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return time.Time{}, nil
	}
	return parseTime(value.String)
}
