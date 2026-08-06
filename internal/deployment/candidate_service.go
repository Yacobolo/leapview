package deployment

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	defaultCandidateLifetime     = 8 * time.Hour
	defaultCandidateQuota        = 4
	CandidateBaseGenerationEmpty = "empty"
)

type CandidateRepository interface {
	ActiveCandidateBaseGeneration(context.Context, string, string) (string, error)
	StartCandidate(context.Context, Candidate, int) (Candidate, bool, error)
	ActiveCandidate(context.Context, string, string, string, string) (Candidate, error)
	CandidateByID(context.Context, string) (Candidate, error)
	SaveCandidate(context.Context, Candidate, int64) (Candidate, error)
	ExpireCandidates(context.Context, string, time.Time) (int64, error)
}

type CandidateServiceConfig struct {
	TargetID          string
	CanonicalOrigin   string
	Environment       string
	Lifetime          time.Duration
	MaxActivePerOwner int
	Now               func() time.Time
	NewID             func() (string, error)
	Audit             func(context.Context, CandidateEvent) error
	RuntimeLifecycle  CandidateRuntimeLifecycle
}

// CandidateRuntimeLifecycle retires private generations after durable
// candidate transitions. Implementations must make both operations idempotent.
type CandidateRuntimeLifecycle interface {
	RetireCandidate(string) int
	ReapExpiredCandidates(time.Time) int
}

type CandidateService struct {
	repository        CandidateRepository
	targetID          string
	canonicalOrigin   string
	environment       string
	lifetime          time.Duration
	maxActivePerOwner int
	now               func() time.Time
	newID             func() (string, error)
	audit             func(context.Context, CandidateEvent) error
	runtimeLifecycle  CandidateRuntimeLifecycle
}

type StartCandidateRequest struct {
	ProjectID      string
	OwnerID        string
	ArtifactDigest string
	Key            string
}

type CandidateStartResult struct {
	Candidate  Candidate
	PreviewURL string
	Resumed    bool
}

type CandidateScope struct {
	ProjectID   string
	CandidateID string
	OwnerID     string
	TargetID    string
}

type CandidateEvent struct {
	Action       string
	CandidateID  string
	ProjectID    string
	TargetID     string
	PrincipalID  string
	Status       CandidateStatus
	MetadataJSON string
}

func NewCandidateService(repository CandidateRepository, config CandidateServiceConfig) (*CandidateService, error) {
	if repository == nil {
		return nil, fmt.Errorf("candidate repository is required")
	}
	config.TargetID = strings.TrimSpace(config.TargetID)
	config.Environment = strings.TrimSpace(config.Environment)
	origin, err := canonicalCandidateOrigin(config.CanonicalOrigin)
	if err != nil {
		return nil, err
	}
	if config.TargetID == "" || config.Environment == "" {
		return nil, fmt.Errorf("candidate target and environment are required")
	}
	if config.Lifetime <= 0 {
		config.Lifetime = defaultCandidateLifetime
	}
	if config.MaxActivePerOwner <= 0 {
		config.MaxActivePerOwner = defaultCandidateQuota
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.NewID == nil {
		config.NewID = newCandidateID
	}
	return &CandidateService{
		repository: repository, targetID: config.TargetID, canonicalOrigin: origin,
		environment: config.Environment, lifetime: config.Lifetime,
		maxActivePerOwner: config.MaxActivePerOwner, now: config.Now, newID: config.NewID, audit: config.Audit,
		runtimeLifecycle: config.RuntimeLifecycle,
	}, nil
}

func (service *CandidateService) Start(ctx context.Context, request StartCandidateRequest) (CandidateStartResult, error) {
	now := service.now().UTC()
	baseGeneration, err := service.repository.ActiveCandidateBaseGeneration(
		ctx, strings.TrimSpace(request.ProjectID), service.environment,
	)
	if err != nil {
		return CandidateStartResult{}, fmt.Errorf("resolve candidate base generation: %w", err)
	}
	id, err := service.newID()
	if err != nil {
		return CandidateStartResult{}, fmt.Errorf("generate candidate id: %w", err)
	}
	candidate, err := NewCandidate(CandidateStartInput{
		ID: id, ProjectID: request.ProjectID, TargetID: service.targetID, Environment: service.environment,
		OwnerID: request.OwnerID, BaseGeneration: baseGeneration, ArtifactDigest: request.ArtifactDigest,
		Key: request.Key, ExpiresAt: now.Add(service.lifetime), Now: now,
	})
	if err != nil {
		return CandidateStartResult{}, err
	}
	candidate, resumed, err := service.repository.StartCandidate(ctx, candidate, service.maxActivePerOwner)
	if err != nil {
		return CandidateStartResult{}, err
	}
	action := "candidate.started"
	if resumed {
		action = "candidate.resumed"
	}
	if err := service.record(ctx, action, candidate, map[string]any{"resumed": resumed}); err != nil {
		return CandidateStartResult{}, err
	}
	return CandidateStartResult{Candidate: candidate, PreviewURL: service.PreviewURL(candidate.ID), Resumed: resumed}, nil
}

func (service *CandidateService) CancelActive(
	ctx context.Context,
	projectID,
	ownerID,
	key string,
) (Candidate, error) {
	candidate, err := service.repository.ActiveCandidate(
		ctx,
		service.targetID,
		strings.TrimSpace(projectID),
		strings.TrimSpace(ownerID),
		normalizeCandidateKey(key),
	)
	if err != nil {
		return Candidate{}, err
	}
	return service.Cancel(ctx, candidateScopeForService(candidate))
}

func candidateScopeForService(candidate Candidate) CandidateScope {
	return CandidateScope{
		ProjectID: candidate.ProjectID, CandidateID: candidate.ID,
		OwnerID: candidate.OwnerID, TargetID: candidate.TargetID,
	}
}

func (service *CandidateService) Get(ctx context.Context, scope CandidateScope) (Candidate, error) {
	candidate, err := service.repository.CandidateByID(ctx, strings.TrimSpace(scope.CandidateID))
	if err != nil {
		return Candidate{}, err
	}
	if !service.owns(candidate, scope) {
		return Candidate{}, ErrCandidateNotFound
	}
	return service.expireOnRead(ctx, candidate)
}

func (service *CandidateService) GetOwned(ctx context.Context, candidateID, ownerID string) (Candidate, error) {
	candidate, err := service.repository.CandidateByID(ctx, strings.TrimSpace(candidateID))
	if err != nil {
		return Candidate{}, err
	}
	if candidate.TargetID != service.targetID || candidate.OwnerID != strings.TrimSpace(ownerID) {
		return Candidate{}, ErrCandidateNotFound
	}
	return service.expireOnRead(ctx, candidate)
}

func (service *CandidateService) Review(
	ctx context.Context,
	projectID,
	candidateID string,
) (Candidate, error) {
	candidate, err := service.repository.CandidateByID(
		ctx,
		strings.TrimSpace(candidateID),
	)
	if err != nil {
		return Candidate{}, err
	}
	if candidate.TargetID != service.targetID ||
		candidate.ProjectID != strings.TrimSpace(projectID) {
		return Candidate{}, ErrCandidateNotFound
	}
	return service.expireOnRead(ctx, candidate)
}

func (service *CandidateService) ReplaceArtifact(ctx context.Context, scope CandidateScope, expectedDigest, nextDigest string) (Candidate, error) {
	return service.mutate(ctx, scope, "candidate.artifact_replaced", func(candidate Candidate) (Candidate, error) {
		now := service.now().UTC()
		return candidate.ReplaceArtifact(expectedDigest, nextDigest, now, now.Add(service.lifetime))
	})
}

func (service *CandidateService) MarkReady(
	ctx context.Context,
	scope CandidateScope,
	artifactDigest,
	provenanceDigest string,
) (Candidate, error) {
	return service.mutate(ctx, scope, "candidate.ready", func(candidate Candidate) (Candidate, error) {
		return candidate.MarkReady(
			artifactDigest,
			provenanceDigest,
			service.now().UTC(),
		)
	})
}

func (service *CandidateService) MarkFailed(ctx context.Context, scope CandidateScope, artifactDigest, failureCode string) (Candidate, error) {
	return service.mutate(ctx, scope, "candidate.failed", func(candidate Candidate) (Candidate, error) {
		return candidate.MarkFailed(artifactDigest, failureCode, service.now().UTC())
	})
}

func (service *CandidateService) Retry(ctx context.Context, scope CandidateScope) (Candidate, error) {
	return service.mutate(ctx, scope, "candidate.retried", func(candidate Candidate) (Candidate, error) {
		now := service.now().UTC()
		return candidate.Retry(now, now.Add(service.lifetime))
	})
}

func (service *CandidateService) Cancel(ctx context.Context, scope CandidateScope) (Candidate, error) {
	candidate, err := service.mutate(ctx, scope, "candidate.cancelled", func(candidate Candidate) (Candidate, error) {
		return candidate.Cancel(service.now().UTC())
	})
	if err == nil && service.runtimeLifecycle != nil {
		service.runtimeLifecycle.RetireCandidate(candidate.ID)
	}
	return candidate, err
}

func (service *CandidateService) Reconcile(ctx context.Context) (int64, error) {
	now := service.now().UTC()
	count, err := service.repository.ExpireCandidates(ctx, service.targetID, now)
	if err != nil {
		return count, err
	}
	if service.runtimeLifecycle != nil {
		service.runtimeLifecycle.ReapExpiredCandidates(now)
	}
	return count, nil
}

func (service *CandidateService) PreviewURL(candidateID string) string {
	return service.canonicalOrigin + "/candidates/" + url.PathEscape(strings.TrimSpace(candidateID))
}

func (service *CandidateService) mutate(
	ctx context.Context,
	scope CandidateScope,
	action string,
	transition func(Candidate) (Candidate, error),
) (Candidate, error) {
	current, err := service.Get(ctx, scope)
	if err != nil {
		return Candidate{}, err
	}
	next, err := transition(current)
	if err != nil {
		return Candidate{}, err
	}
	if next == current {
		return current, nil
	}
	saved, err := service.repository.SaveCandidate(ctx, next, current.Revision)
	if err != nil {
		return Candidate{}, err
	}
	if err := service.record(ctx, action, saved, nil); err != nil {
		return Candidate{}, err
	}
	return saved, nil
}

func (service *CandidateService) owns(candidate Candidate, scope CandidateScope) bool {
	targetID := strings.TrimSpace(scope.TargetID)
	if targetID == "" {
		targetID = service.targetID
	}
	return strings.TrimSpace(scope.CandidateID) == candidate.ID &&
		strings.TrimSpace(scope.ProjectID) == candidate.ProjectID &&
		strings.TrimSpace(scope.OwnerID) == candidate.OwnerID &&
		targetID == service.targetID && candidate.TargetID == service.targetID
}

func (service *CandidateService) expireOnRead(ctx context.Context, candidate Candidate) (Candidate, error) {
	now := service.now().UTC()
	for attempt := 0; attempt < 2; attempt++ {
		expired, changed, err := candidate.Expire(now)
		if err != nil || !changed {
			return expired, err
		}
		saved, err := service.repository.SaveCandidate(ctx, expired, candidate.Revision)
		if err == nil {
			if err := service.record(ctx, "candidate.expired", saved, nil); err != nil {
				return Candidate{}, err
			}
			if service.runtimeLifecycle != nil {
				service.runtimeLifecycle.RetireCandidate(saved.ID)
			}
			return saved, nil
		}
		if !errors.Is(err, ErrCandidateConflict) {
			return Candidate{}, err
		}
		candidate, err = service.repository.CandidateByID(ctx, candidate.ID)
		if err != nil {
			return Candidate{}, err
		}
	}
	return Candidate{}, ErrCandidateConflict
}

func (service *CandidateService) record(ctx context.Context, action string, candidate Candidate, metadata map[string]any) error {
	if service.audit == nil {
		return nil
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["environment"] = candidate.Environment
	metadata["baseGeneration"] = candidate.BaseGeneration
	metadata["projectId"] = candidate.ProjectID
	metadata["candidateKey"] = candidate.Key
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return service.audit(ctx, CandidateEvent{
		Action: action, CandidateID: candidate.ID, ProjectID: candidate.ProjectID, TargetID: candidate.TargetID,
		PrincipalID: candidate.OwnerID, Status: candidate.Status, MetadataJSON: string(encoded),
	})
}

func canonicalCandidateOrigin(value string) (string, error) {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	origin, err := url.Parse(value)
	if err != nil || (origin.Scheme != "https" && origin.Scheme != "http") || origin.Host == "" ||
		(origin.Path != "" && origin.Path != "/") || origin.RawQuery != "" || origin.Fragment != "" || origin.User != nil {
		return "", fmt.Errorf("candidate canonical origin must be an HTTP(S) origin")
	}
	return origin.Scheme + "://" + origin.Host, nil
}

func newCandidateID() (string, error) {
	var entropy [24]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}
	return "cand_" + base64.RawURLEncoding.EncodeToString(entropy[:]), nil
}
