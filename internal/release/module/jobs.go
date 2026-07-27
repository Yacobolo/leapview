package module

import (
	"context"
	"encoding/json"

	"github.com/Yacobolo/leapview/internal/platform/jobs"
	"github.com/Yacobolo/leapview/internal/release"
)

const FinalizeJobKind = "release.finalize"

type FinalizeJob struct {
	Project string
	Release string
}

func (m *Module) JobHandlers(events jobs.EventAppender) []jobs.Handler {
	return []jobs.Handler{jobs.HandlerFunc{JobKind: FinalizeJobKind, Run: func(ctx context.Context, job jobs.Job) error {
		var payload FinalizeJob
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return err
		}
		row, err := m.service.ValidateFinalization(ctx, payload.Project, payload.Release)
		event := "release.ready"
		if err != nil {
			event = "release.failed"
		}
		data, _ := json.Marshal(finalizationEvent(row))
		_, _ = events.AppendEvent(context.WithoutCancel(ctx), "release", payload.Release, event, data)
		return err
	}}}
}

func finalizationEvent(row release.Release) map[string]any {
	workspaces := make([]map[string]any, 0, len(row.Manifest.Workspaces))
	for _, item := range row.Manifest.Workspaces {
		mapped := map[string]any{"workspace": item.WorkspaceID, "artifactDigest": item.ArtifactDigest}
		if item.ServingStateID != "" {
			mapped["servingStateId"] = item.ServingStateID
		}
		workspaces = append(workspaces, mapped)
	}
	connections := make([]map[string]any, 0, len(row.Manifest.Connections))
	for _, item := range row.Manifest.Connections {
		connections = append(connections, map[string]any{"connection": item.ConnectionID, "revisionId": item.RevisionID})
	}
	result := map[string]any{
		"id": row.ID, "projectId": row.ProjectID, "projectDigest": row.ProjectDigest,
		"status": string(row.Status), "createdBy": row.CreatedBy, "createdAt": row.CreatedAt,
		"workspaces": workspaces, "connections": connections,
	}
	if row.FinalizedAt != "" {
		result["finalizedAt"] = row.FinalizedAt
	}
	if row.Error != "" {
		result["error"] = row.Error
	}
	return result
}
