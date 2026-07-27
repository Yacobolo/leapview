package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Yacobolo/leapview/internal/platform/jobs"
	platformdb "github.com/Yacobolo/leapview/internal/platform/jobs/sqlite/jobdb"
	"github.com/Yacobolo/leapview/internal/platform/transaction"
)

type Repository struct{ q *platformdb.Queries }

func NewRepository(db platformdb.DBTX) *Repository { return &Repository{q: platformdb.New(db)} }

func (r *Repository) Enqueue(ctx context.Context, input jobs.EnqueueInput) (jobs.Job, error) {
	input.ID, input.Kind = strings.TrimSpace(input.ID), strings.TrimSpace(input.Kind)
	input.WorkloadClass, input.WorkspaceID = strings.TrimSpace(input.WorkloadClass), strings.TrimSpace(input.WorkspaceID)
	input.ResourceKind, input.ResourceID = strings.TrimSpace(input.ResourceKind), strings.TrimSpace(input.ResourceID)
	if input.ID == "" || input.Kind == "" || input.WorkloadClass == "" || input.WorkspaceID == "" || input.ResourceKind == "" || input.ResourceID == "" || !json.Valid(input.Payload) {
		return jobs.Job{}, fmt.Errorf("invalid async job")
	}
	digest := jobDigest(input)
	err := r.q.EnqueueAPIAsyncJob(ctx, platformdb.EnqueueAPIAsyncJobParams{ID: input.ID, JobKind: input.Kind, WorkloadClass: input.WorkloadClass, WorkspaceID: input.WorkspaceID,
		ResourceKind: input.ResourceKind, ResourceID: input.ResourceID, PayloadJson: string(input.Payload), RequestDigest: digest})
	if err != nil {
		existing, getErr := r.Get(ctx, input.ID)
		if getErr == nil {
			if storedDigest, scanErr := r.q.GetAPIAsyncJobDigest(ctx, input.ID); scanErr == nil && storedDigest == digest {
				return existing, nil
			}
			return jobs.Job{}, jobs.ErrConflict
		}
		return jobs.Job{}, err
	}
	return r.Get(ctx, input.ID)
}

func jobDigest(input jobs.EnqueueInput) string {
	sum := sha256.Sum256([]byte(input.Kind + "\x00" + input.WorkloadClass + "\x00" + input.WorkspaceID + "\x00" + input.ResourceKind + "\x00" + input.ResourceID + "\x00" + string(input.Payload)))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (r *Repository) Get(ctx context.Context, id string) (jobs.Job, error) {
	row, err := r.q.GetAPIAsyncJob(ctx, strings.TrimSpace(id))
	if errors.Is(err, sql.ErrNoRows) {
		return jobs.Job{}, jobs.ErrNotFound
	}
	return jobFromGetRow(row), err
}

func (r *Repository) Candidates(ctx context.Context, workloadClass string, limit int) ([]jobs.Job, error) {
	workloadClass = strings.TrimSpace(workloadClass)
	if workloadClass == "" || limit < 1 || limit > 200 {
		return nil, fmt.Errorf("workload class and candidate limit are required")
	}
	rows, err := r.q.ListAPIAsyncJobCandidates(ctx, platformdb.ListAPIAsyncJobCandidatesParams{WorkloadClass: workloadClass, ResultLimit: int64(limit)})
	if err != nil {
		return nil, err
	}
	jobs := make([]jobs.Job, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, jobFromCandidateRow(row))
	}
	return jobs, nil
}

func (r *Repository) ClaimByID(ctx context.Context, id, workloadClass, owner string, lease time.Duration) (jobs.Job, bool, error) {
	id, workloadClass, owner = strings.TrimSpace(id), strings.TrimSpace(workloadClass), strings.TrimSpace(owner)
	if id == "" || workloadClass == "" || owner == "" || lease <= 0 {
		return jobs.Job{}, false, fmt.Errorf("job id, workload class, worker owner, and positive lease are required")
	}
	modifier := fmt.Sprintf("+%d seconds", max(1, int(lease.Seconds())))
	row, err := r.q.ClaimAPIAsyncJobByID(ctx, platformdb.ClaimAPIAsyncJobByIDParams{ID: id, WorkloadClass: workloadClass, LeaseOwner: owner, LeaseModifier: modifier})
	if errors.Is(err, sql.ErrNoRows) {
		return jobs.Job{}, false, nil
	}
	job := jobFromClaimRow(row)
	return job, err == nil, err
}

func (r *Repository) Renew(ctx context.Context, id string, fence jobs.Fence, lease time.Duration) error {
	modifier := fmt.Sprintf("+%d seconds", max(1, int(lease.Seconds())))
	changed, err := r.q.RenewAPIAsyncJob(ctx, platformdb.RenewAPIAsyncJobParams{
		LeaseModifier: modifier, ID: strings.TrimSpace(id), LeaseOwner: strings.TrimSpace(fence.Owner), LeaseGeneration: fence.Generation,
	})
	return requireChanged(changed, err)
}

func (r *Repository) Complete(ctx context.Context, id string, fence jobs.Fence) error {
	changed, err := r.q.CompleteAPIAsyncJob(ctx, platformdb.CompleteAPIAsyncJobParams{
		ID: strings.TrimSpace(id), LeaseOwner: strings.TrimSpace(fence.Owner), LeaseGeneration: fence.Generation,
	})
	return requireChanged(changed, err)
}

func (r *Repository) Fail(ctx context.Context, id string, fence jobs.Fence, problem []byte) error {
	if !json.Valid(problem) {
		problem = []byte(`{"code":"ASYNC_JOB_FAILED"}`)
	}
	changed, err := r.q.FailAPIAsyncJob(ctx, platformdb.FailAPIAsyncJobParams{
		ErrorJson: string(problem), ID: strings.TrimSpace(id), LeaseOwner: strings.TrimSpace(fence.Owner), LeaseGeneration: fence.Generation,
	})
	return requireChanged(changed, err)
}

func (r *Repository) Cancel(ctx context.Context, id string) error {
	changed, err := r.q.CancelQueuedAPIAsyncJob(ctx, strings.TrimSpace(id))
	return requireChanged(changed, err)
}

func (r *Repository) CancelClaimed(ctx context.Context, id string, fence jobs.Fence) error {
	changed, err := r.q.CancelClaimedAPIAsyncJob(ctx, platformdb.CancelClaimedAPIAsyncJobParams{
		ID: strings.TrimSpace(id), LeaseOwner: strings.TrimSpace(fence.Owner), LeaseGeneration: fence.Generation,
	})
	return requireChanged(changed, err)
}

func (r *Repository) AppendEvent(ctx context.Context, resourceKind, resourceID, eventType string, data []byte) (jobs.Event, error) {
	resourceKind, resourceID, eventType = strings.TrimSpace(resourceKind), strings.TrimSpace(resourceID), strings.TrimSpace(eventType)
	if resourceKind == "" || resourceID == "" || eventType == "" || !json.Valid(data) {
		return jobs.Event{}, fmt.Errorf("invalid async event")
	}
	row, err := r.q.AppendAPIAsyncEvent(ctx, platformdb.AppendAPIAsyncEventParams{ResourceKind: resourceKind, ResourceID: resourceID, EventType: eventType, DataJson: string(data)})
	if err != nil {
		return jobs.Event{}, err
	}
	return eventFromValues(row.EventID, row.ResourceKind, row.ResourceID, row.EventType, row.DataJson, row.CreatedAt), nil
}

func (r *Repository) RecordWorkflow(ctx context.Context, tx transaction.Transaction, intent jobs.WorkflowIntent) error {
	if tx == nil {
		return fmt.Errorf("workflow transaction is required")
	}
	event := intent.Event
	event.Key, event.ResourceKind = strings.TrimSpace(event.Key), strings.TrimSpace(event.ResourceKind)
	event.ResourceID, event.EventType = strings.TrimSpace(event.ResourceID), strings.TrimSpace(event.EventType)
	if event.Key == "" || event.ResourceKind == "" || event.ResourceID == "" || event.EventType == "" || !json.Valid(event.Data) {
		return fmt.Errorf("invalid workflow event")
	}
	transactional := NewRepository(tx)
	if _, err := transactional.q.AppendAPIAsyncWorkflowEvent(ctx, platformdb.AppendAPIAsyncWorkflowEventParams{
		ResourceKind: event.ResourceKind, ResourceID: event.ResourceID, EventType: event.EventType,
		DataJson: string(event.Data), EventKey: event.Key,
	}); err != nil {
		return err
	}
	_, err := transactional.Enqueue(ctx, intent.Job)
	return err
}

func (r *Repository) ListEvents(ctx context.Context, resourceKind, resourceID string, after int64, limit int) ([]jobs.Event, error) {
	if limit < 1 || limit > 200 {
		return nil, fmt.Errorf("event limit must be between 1 and 200")
	}
	rows, err := r.q.ListAPIAsyncEvents(ctx, platformdb.ListAPIAsyncEventsParams{ResourceKind: resourceKind, ResourceID: resourceID, EventID: after, Limit: int64(limit)})
	if err != nil {
		return nil, err
	}
	events := make([]jobs.Event, 0, len(rows))
	for _, row := range rows {
		event := eventFromValues(row.EventID, row.ResourceKind, row.ResourceID, row.EventType, row.DataJson, row.CreatedAt)
		events = append(events, event)
	}
	return events, nil
}

func (r *Repository) event(ctx context.Context, kind, id string, eventID int64) (jobs.Event, error) {
	row, err := r.q.GetAPIAsyncEvent(ctx, platformdb.GetAPIAsyncEventParams{ResourceKind: kind, ResourceID: id, EventID: eventID})
	return eventFromValues(row.EventID, row.ResourceKind, row.ResourceID, row.EventType, row.DataJson, row.CreatedAt), err
}

func jobFromGetRow(row platformdb.GetAPIAsyncJobRow) jobs.Job {
	return jobs.Job{ID: row.ID, Kind: row.JobKind, WorkloadClass: row.WorkloadClass, WorkspaceID: row.WorkspaceID, ResourceKind: row.ResourceKind, ResourceID: row.ResourceID,
		Payload: []byte(row.PayloadJson), Status: jobs.Status(row.Status), Attempts: int(row.AttemptCount), LeaseOwner: row.LeaseOwner, LeaseGeneration: row.LeaseGeneration,
		LeaseExpiresAt: row.LeaseExpiresAt, CreatedAt: row.CreatedAt, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt, ErrorJSON: row.ErrorJson}
}

func jobFromClaimRow(row platformdb.ClaimAPIAsyncJobByIDRow) jobs.Job {
	return jobs.Job{ID: row.ID, Kind: row.JobKind, WorkloadClass: row.WorkloadClass, WorkspaceID: row.WorkspaceID, ResourceKind: row.ResourceKind, ResourceID: row.ResourceID,
		Payload: []byte(row.PayloadJson), Status: jobs.Status(row.Status), Attempts: int(row.AttemptCount), LeaseOwner: row.LeaseOwner, LeaseGeneration: row.LeaseGeneration,
		LeaseExpiresAt: row.LeaseExpiresAt, CreatedAt: row.CreatedAt, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt, ErrorJSON: row.ErrorJson}
}

func jobFromCandidateRow(row platformdb.ListAPIAsyncJobCandidatesRow) jobs.Job {
	return jobs.Job{ID: row.ID, Kind: row.JobKind, WorkloadClass: row.WorkloadClass, WorkspaceID: row.WorkspaceID, ResourceKind: row.ResourceKind, ResourceID: row.ResourceID,
		Payload: []byte(row.PayloadJson), Status: jobs.Status(row.Status), Attempts: int(row.AttemptCount), LeaseOwner: row.LeaseOwner, LeaseGeneration: row.LeaseGeneration,
		LeaseExpiresAt: row.LeaseExpiresAt, CreatedAt: row.CreatedAt, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt, ErrorJSON: row.ErrorJson}
}

func eventFromValues(eventID int64, kind, id, eventType, data, createdAt string) jobs.Event {
	return jobs.Event{ID: eventID, ResourceKind: kind, ResourceID: id, EventType: eventType, Data: []byte(data), CreatedAt: createdAt}
}

func requireChanged(changed int64, err error) error {
	if err != nil {
		return err
	}
	if changed != 1 {
		return jobs.ErrConflict
	}
	return nil
}
