package connectionbinding

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type AdministrationAuditAction string

const (
	AuditBindingCreated  AdministrationAuditAction = "connection.binding.created"
	AuditBindingUpdated  AdministrationAuditAction = "connection.binding.updated"
	AuditBindingEnabled  AdministrationAuditAction = "connection.binding.enabled"
	AuditBindingDisabled AdministrationAuditAction = "connection.binding.disabled"
)

type AdministrationAuditOutcome string

const AdministrationAuditSucceeded AdministrationAuditOutcome = "succeeded"

type AdministrationAuditEvent struct {
	WorkspaceID         string                     `json:"workspaceId"`
	BindingID           string                     `json:"bindingId"`
	TargetID            string                     `json:"targetId"`
	LogicalConnectionID LogicalConnectionID        `json:"logicalConnection"`
	Actor               string                     `json:"actor"`
	Action              AdministrationAuditAction  `json:"action"`
	Outcome             AdministrationAuditOutcome `json:"outcome"`
	Revision            int64                      `json:"revision"`
	Timestamp           time.Time                  `json:"timestamp"`
}

type AdministrationAuditRecorder interface {
	RecordConnectionAdministration(context.Context, AdministrationAuditEvent) error
}

func (service *Administration) recordMutation(
	ctx context.Context,
	actor string,
	action AdministrationAuditAction,
	binding TargetBinding,
) error {
	if service == nil || service.audit == nil {
		return nil
	}
	event := AdministrationAuditEvent{
		WorkspaceID: binding.Scope.WorkspaceID, BindingID: binding.ID,
		TargetID: binding.TargetID, LogicalConnectionID: binding.LogicalConnectionID,
		Actor: strings.TrimSpace(actor), Action: action, Outcome: AdministrationAuditSucceeded,
		Revision: binding.Revision, Timestamp: service.now().UTC(),
	}
	if err := service.audit.RecordConnectionAdministration(context.WithoutCancel(ctx), event); err != nil {
		return fmt.Errorf("%w: %v", ErrAdministrationAuditUnavailable, err)
	}
	return nil
}
