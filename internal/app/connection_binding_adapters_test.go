package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	accessmodule "github.com/flidai/leapview/internal/access/module"
	analyticsmodule "github.com/flidai/leapview/internal/analytics/module"
)

func TestConnectionRotationAuditAdapterPersistsOnlyRedactedBoundedMetadata(t *testing.T) {
	var input accessmodule.AuditEventInput
	recorder := connectionRotationAuditRecorder{
		record: func(_ context.Context, current accessmodule.AuditEventInput) error {
			input = current
			return nil
		},
	}
	err := recorder.RecordCredentialRotation(context.Background(), analyticsmodule.ConnectionRotationAuditEvent{
		BindingID: "binding_prod_warehouse", TargetID: "lvinst_prod", WorkspaceID: "sales",
		ProviderVersion: "secret:v2", Actor: "principal:operator-1",
		Operation: "credential.test.requested", Outcome: "degraded",
		Reason: "POOL_HEALTH_CHECK_FAILED", Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if input.WorkspaceID != "sales" || input.PrincipalID != "operator-1" ||
		input.Action != "credential.test.requested" ||
		input.TargetType != "connection_binding" || input.TargetID != "binding_prod_warehouse" ||
		input.Privilege != accessmodule.PrivilegeTestConnection || input.Status != "degraded" {
		t.Fatalf("audit input = %#v", input)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(input.MetadataJSON), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["diagnosticCode"] != "POOL_HEALTH_CHECK_FAILED" ||
		metadata["providerVersion"] != "secret:v2" ||
		metadata["targetId"] != "lvinst_prod" {
		t.Fatalf("audit metadata = %#v", metadata)
	}
	for _, forbidden := range []string{"source-secret", "connection_string", "password", "/leapview/sales"} {
		if strings.Contains(input.MetadataJSON, forbidden) {
			t.Fatalf("audit metadata disclosed %q: %s", forbidden, input.MetadataJSON)
		}
	}
}

func TestConnectionAdministrationAuditAdapterPersistsOnlyBindingIdentity(t *testing.T) {
	var input accessmodule.AuditEventInput
	recorder := connectionAdministrationAuditRecorder{
		record: func(_ context.Context, current accessmodule.AuditEventInput) error {
			input = current
			return nil
		},
	}
	err := recorder.RecordConnectionAdministration(context.Background(), analyticsmodule.ConnectionAdministrationAuditEvent{
		WorkspaceID: "sales", BindingID: "binding_prod_warehouse", TargetID: "lvinst_prod",
		LogicalConnectionID: "warehouse", Actor: "operator-1",
		Action: "connection.binding.updated", Outcome: "succeeded", Revision: 7,
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if input.WorkspaceID != "sales" || input.PrincipalID != "operator-1" ||
		input.Action != "connection.binding.updated" ||
		input.TargetType != "connection_binding" || input.TargetID != "binding_prod_warehouse" ||
		input.Privilege != accessmodule.PrivilegeManageConnectionMetadata || input.Status != "succeeded" {
		t.Fatalf("audit input = %#v", input)
	}
	for _, forbidden := range []string{"source-secret", "connection_string", "password", "secretPath"} {
		if strings.Contains(input.MetadataJSON, forbidden) {
			t.Fatalf("audit metadata disclosed %q: %s", forbidden, input.MetadataJSON)
		}
	}
}
