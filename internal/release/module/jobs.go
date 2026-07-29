package module

import (
	"context"
	"encoding/json"

	"github.com/flidai/leapview/internal/platform/jobs"
)

const FinalizeJobKind = "release.finalize"

type FinalizeJob struct {
	Project string
	Release string
}

func (m *Module) JobHandlers() []jobs.Handler {
	return []jobs.Handler{jobs.HandlerFunc{JobKind: FinalizeJobKind, Run: func(ctx context.Context, job jobs.Job) error {
		var payload FinalizeJob
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return err
		}
		_, err := m.service.ValidateFinalization(ctx, payload.Project, payload.Release)
		return err
	}}}
}
