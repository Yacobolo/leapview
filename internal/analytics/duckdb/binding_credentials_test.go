package duckdb

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/flidai/leapview/internal/analytics/connectionbinding"
	semanticmodel "github.com/flidai/leapview/internal/analytics/model"
	"github.com/stretchr/testify/require"
)

func TestApplyTargetBindingBuildsBoundedValidatedRuntimeConnection(t *testing.T) {
	binding := testDuckDBTargetBinding(t)
	snapshot, err := connectionbinding.NewCredentialSnapshot(
		map[string]string{"password": "source-secret"},
		"secret-1:v4", time.Now(), time.Now().Add(time.Minute),
	)
	require.NoError(t, err)
	connection, err := ApplyTargetBinding(
		semanticmodel.Connection{Kind: "postgres"},
		binding,
		snapshot,
	)
	require.NoError(t, err)
	if connection.Host != "warehouse.internal" || connection.Port != 5432 ||
		connection.Database != "analytics" || connection.Username != "leapview_runtime" ||
		connection.SSLMode != "verify-full" {
		t.Fatalf("runtime endpoint = %#v", connection)
	}
	if connection.Auth["password"] != "source-secret" {
		t.Fatal("runtime auth bundle was not applied")
	}
}

func TestApplyTargetBindingFailsClosedWithoutDisclosingInvalidBundle(t *testing.T) {
	snapshot, err := connectionbinding.NewCredentialSnapshot(
		map[string]string{"api_token": "source-secret"},
		"secret-1:v5", time.Now(), time.Now().Add(time.Minute),
	)
	require.NoError(t, err)
	_, err = ApplyTargetBinding(semanticmodel.Connection{Kind: "postgres"}, testDuckDBTargetBinding(t), snapshot)
	if !errors.Is(err, connectionbinding.ErrInvalidCredentialBundle) || strings.Contains(err.Error(), "source-secret") {
		t.Fatalf("ApplyTargetBinding() error = %v", err)
	}
}

func testDuckDBTargetBinding(t *testing.T) connectionbinding.TargetBinding {
	t.Helper()
	binding, err := connectionbinding.NewTargetBinding(connectionbinding.TargetBindingInput{
		ID: "binding_prod_warehouse", TargetID: "lvinst_prod", LogicalConnectionID: "warehouse",
		ConnectorKind: "postgres", AuthenticationMode: connectionbinding.AuthenticationExternalBundle,
		Scope: connectionbinding.BindingScope{WorkspaceID: "sales", Environment: "prod"},
		Endpoint: connectionbinding.EndpointConfig{
			Host: "warehouse.internal", Port: 5432, Database: "analytics",
			SourceIdentity: "leapview_runtime", TLSMode: "verify-full",
		},
		CredentialReference: connectionbinding.CredentialReference{
			ProjectID: "project-1", Environment: "prod", SecretPath: "/leapview/sales", SecretKey: "warehouse",
		},
		Enabled: true, Now: time.Now(),
	})
	require.NoError(t, err)
	return binding
}
