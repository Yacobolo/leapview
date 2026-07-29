package connectionbinding

import (
	"context"
	"strings"
	"time"
)

type RefreshOperation string

const (
	RefreshScheduled RefreshOperation = "credential.refresh.scheduled"
	RefreshRequested RefreshOperation = "credential.refresh.requested"
)

type RotationOutcome string

const (
	RotationActivated RotationOutcome = "activated"
	RotationUnchanged RotationOutcome = "unchanged"
	RotationDegraded  RotationOutcome = "degraded"
)

type RefreshRequest struct {
	Actor     string
	Operation RefreshOperation
}

func (request RefreshRequest) valid() bool {
	request.Actor = strings.TrimSpace(request.Actor)
	if request.Actor == "" || len(request.Actor) > 256 {
		return false
	}
	return request.Operation == RefreshScheduled || request.Operation == RefreshRequested
}

type RotationAuditEvent struct {
	BindingID       string           `json:"bindingId"`
	TargetID        string           `json:"targetId"`
	ProviderVersion string           `json:"providerVersion,omitempty"`
	Actor           string           `json:"actor"`
	Operation       RefreshOperation `json:"operation"`
	Timestamp       time.Time        `json:"timestamp"`
	Outcome         RotationOutcome  `json:"outcome"`
	Reason          string           `json:"reason,omitempty"`
}

type RotationAuditRecorder interface {
	RecordCredentialRotation(context.Context, RotationAuditEvent) error
}
