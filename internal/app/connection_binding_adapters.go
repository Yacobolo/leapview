package app

import (
	"context"
	"encoding/json"
	"strings"

	accessmodule "github.com/flidai/leapview/internal/access/module"
	analyticsmodule "github.com/flidai/leapview/internal/analytics/module"
)

// connectionBindingDependenciesWithoutConsumers is the composition adapter
// for the current serving graph. Target-owned bindings are not yet referenced
// by compiled serving states, so there are no registered dependents to report.
// The adapter keeps that fact explicit at the capability boundary.
type connectionBindingDependenciesWithoutConsumers struct{}

func (connectionBindingDependenciesWithoutConsumers) Dependents(
	context.Context,
	analyticsmodule.ConnectionTargetBinding,
) ([]analyticsmodule.ConnectionBindingDependency, error) {
	return nil, nil
}

type connectionRotationAuditRecorder struct {
	record func(context.Context, accessmodule.AuditEventInput) error
}

func (recorder connectionRotationAuditRecorder) RecordCredentialRotation(
	ctx context.Context,
	event analyticsmodule.ConnectionRotationAuditEvent,
) error {
	metadata, _ := json.Marshal(map[string]any{
		"operation":       event.Operation,
		"outcome":         event.Outcome,
		"providerVersion": event.ProviderVersion,
		"diagnosticCode":  event.Reason,
		"targetId":        event.TargetID,
	})
	principalID := strings.TrimPrefix(event.Actor, "principal:")
	if strings.HasPrefix(event.Actor, "runtime:") {
		principalID = ""
	}
	if recorder.record == nil {
		return nil
	}
	return recorder.record(ctx, accessmodule.AuditEventInput{
		WorkspaceID: event.WorkspaceID, PrincipalID: principalID,
		Action:       string(event.Operation),
		TargetType:   "connection_binding",
		TargetID:     event.BindingID,
		Privilege:    accessmodule.PrivilegeTestConnection,
		Status:       string(event.Outcome),
		MetadataJSON: string(metadata),
	})
}
