package module

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/Yacobolo/leapview/internal/deployment/apiadapter"
	"github.com/Yacobolo/leapview/internal/platform/jobs"
)

const ActivateJobKind = "deployment.activate"

type ActivateJob struct {
	Project        string
	Deployment     string
	Actor          string
	IdempotencyKey string
}

type DeploymentCoordinator interface {
	Create(context.Context, apiadapter.CreateRequest) (apiadapter.Deployment, error)
	Get(context.Context, apiadapter.Scope) (apiadapter.Deployment, error)
	Activate(context.Context, apiadapter.ActivateRequest) (apiadapter.Deployment, error)
	Cancel(context.Context, apiadapter.Scope) (apiadapter.Deployment, error)
}

// JobConfig contains deployment-owned workflow ports. Authorization is a
// consumer-defined port; schedule reconciliation is an explicit downstream
// notification rather than repository reach-through.
type JobConfig struct {
	Coordinator DeploymentCoordinator
	Authorize   func(context.Context, string, string, []apiadapter.TargetRequest) error
	Reconcile   func(context.Context) error
	Events      jobs.EventAppender
	Logger      *slog.Logger
}

func (m *Module) JobHandlers() []jobs.Handler {
	return []jobs.Handler{jobs.HandlerFunc{JobKind: ActivateJobKind, Run: m.activate}}
}

func (m *Module) activate(ctx context.Context, job jobs.Job) error {
	if m.jobs.Coordinator == nil {
		return fmt.Errorf("deployment coordinator is unavailable")
	}
	var payload ActivateJob
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return err
	}
	pending, err := m.jobs.Coordinator.Get(ctx, apiadapter.Scope{Project: payload.Project, DeploymentID: payload.Deployment})
	if err != nil {
		return err
	}
	targets := make([]apiadapter.TargetRequest, 0, len(pending.Targets))
	for _, target := range pending.Targets {
		targets = append(targets, apiadapter.TargetRequest{Workspace: target.Workspace, CandidateID: target.CandidateID})
	}
	if m.jobs.Authorize != nil {
		if err := m.jobs.Authorize(ctx, payload.Actor, pending.Environment, targets); err != nil {
			m.appendEvent(ctx, payload.Deployment, "deployment.failed", "failed")
			return err
		}
	}
	row, err := m.jobs.Coordinator.Activate(ctx, apiadapter.ActivateRequest{
		Scope: apiadapter.Scope{Project: payload.Project, DeploymentID: payload.Deployment},
		Actor: payload.Actor, IdempotencyKey: payload.IdempotencyKey,
	})
	if err == nil && m.jobs.Reconcile != nil {
		if reconcileErr := m.jobs.Reconcile(ctx); reconcileErr != nil {
			logger := m.jobs.Logger
			if logger == nil {
				logger = slog.Default()
			}
			logger.WarnContext(ctx, "reconcile refresh pipelines after deployment activation failed", "error", reconcileErr)
		}
	}
	event := "deployment.active"
	if err != nil {
		event = "deployment.failed"
	}
	m.appendEvent(ctx, payload.Deployment, event, string(row.Status))
	return err
}

func (m *Module) appendEvent(ctx context.Context, deploymentID, event, status string) {
	if m.jobs.Events == nil {
		return
	}
	data, _ := json.Marshal(map[string]any{"deploymentId": deploymentID, "status": status})
	_, _ = m.jobs.Events.AppendEvent(context.WithoutCancel(ctx), "deployment", deploymentID, event, data)
}
