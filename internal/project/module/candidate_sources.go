package module

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/flidai/leapview/internal/project"
	projectdevloop "github.com/flidai/leapview/internal/project/devloop"
)

const candidateSourcePlanLifetime = 5 * time.Minute

type candidateSourceSynchronizer struct {
	store *projectdevloop.TargetStore
	now   func() time.Time
	mu    sync.Mutex
	plans map[candidateSourcePlanKey]candidateSourcePlan
}

type candidateSourcePlanKey struct {
	projectID      string
	ownerID        string
	artifactDigest string
}

type candidateSourcePlan struct {
	expiresAt time.Time
	missing   map[string]struct{}
}

func NewCandidateSourceSynchronizer(root string) (project.CandidateSourceSynchronizer, error) {
	store, err := projectdevloop.NewTargetStore(root)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", project.ErrCandidateSourceUnavailable, err)
	}
	return &candidateSourceSynchronizer{
		store: store, now: time.Now,
		plans: make(map[candidateSourcePlanKey]candidateSourcePlan),
	}, nil
}

func (synchronizer *candidateSourceSynchronizer) Plan(
	ctx context.Context,
	scope project.CandidateSourceScope,
	request project.CandidateSynchronizationRequest,
) ([]string, error) {
	if synchronizer == nil || synchronizer.store == nil {
		return nil, project.ErrCandidateSourceUnavailable
	}
	missing, err := synchronizer.store.Missing(ctx, synchronizationPlanRequest(scope, request))
	if err != nil {
		return nil, candidateSourceInvalid(err)
	}
	synchronizer.mu.Lock()
	defer synchronizer.mu.Unlock()
	synchronizer.purgeExpiredLocked()
	allowed := make(map[string]struct{}, len(missing))
	for _, identity := range missing {
		allowed[identity] = struct{}{}
	}
	synchronizer.plans[candidateSourcePlanKey{
		projectID: strings.TrimSpace(scope.ProjectID), ownerID: strings.TrimSpace(scope.OwnerID),
		artifactDigest: strings.TrimSpace(request.ArtifactDigest),
	}] = candidateSourcePlan{
		expiresAt: synchronizer.now().UTC().Add(candidateSourcePlanLifetime), missing: allowed,
	}
	return missing, nil
}

func (synchronizer *candidateSourceSynchronizer) Upload(
	ctx context.Context,
	scope project.CandidateSourceScope,
	identity string,
	source io.Reader,
) error {
	if synchronizer == nil || synchronizer.store == nil {
		return project.ErrCandidateSourceUnavailable
	}
	projectID := strings.TrimSpace(scope.ProjectID)
	ownerID := strings.TrimSpace(scope.OwnerID)
	identity = strings.TrimSpace(identity)
	synchronizer.mu.Lock()
	synchronizer.purgeExpiredLocked()
	authorized := false
	for key, plan := range synchronizer.plans {
		if key.projectID != projectID || key.ownerID != ownerID {
			continue
		}
		if _, exists := plan.missing[identity]; exists {
			authorized = true
			break
		}
	}
	synchronizer.mu.Unlock()
	if !authorized {
		return fmt.Errorf(
			"%w: source blob was not requested by an active synchronization plan",
			project.ErrCandidateSourceConflict,
		)
	}
	if err := synchronizer.store.Put(ctx, identity, source); err != nil {
		return candidateSourceInvalid(err)
	}
	return nil
}

func (synchronizer *candidateSourceSynchronizer) Commit(
	ctx context.Context,
	scope project.CandidateSourceScope,
	request project.CandidateSynchronizationRequest,
) (project.CandidateSourceSnapshot, error) {
	if synchronizer == nil || synchronizer.store == nil {
		return project.CandidateSourceSnapshot{}, project.ErrCandidateSourceUnavailable
	}
	stored, err := synchronizer.store.Commit(ctx, synchronizationPlanRequest(scope, request))
	if err != nil {
		return project.CandidateSourceSnapshot{}, candidateSourceInvalid(err)
	}
	synchronizer.mu.Lock()
	delete(synchronizer.plans, candidateSourcePlanKey{
		projectID: strings.TrimSpace(scope.ProjectID), ownerID: strings.TrimSpace(scope.OwnerID),
		artifactDigest: strings.TrimSpace(request.ArtifactDigest),
	})
	synchronizer.mu.Unlock()
	return project.CandidateSourceSnapshot{
		ProjectID: stored.ProjectID, ArtifactDigest: stored.Digest,
		ProjectPath: stored.ProjectPath,
	}, nil
}

func (synchronizer *candidateSourceSynchronizer) purgeExpiredLocked() {
	now := synchronizer.now().UTC()
	for key, plan := range synchronizer.plans {
		if !now.Before(plan.expiresAt) {
			delete(synchronizer.plans, key)
		}
	}
}

func synchronizationPlanRequest(
	scope project.CandidateSourceScope,
	request project.CandidateSynchronizationRequest,
) projectdevloop.SynchronizationPlanRequest {
	result := projectdevloop.SynchronizationPlanRequest{
		ProjectID: strings.TrimSpace(scope.ProjectID), ProjectFile: request.ProjectFile,
		ArtifactDigest:         request.ArtifactDigest,
		ExpectedCandidateID:    request.ExpectedCandidateID,
		ExpectedArtifactDigest: request.ExpectedArtifactDigest,
		Artifacts:              make([]projectdevloop.ArtifactReference, len(request.Artifacts)),
	}
	for index, artifact := range request.Artifacts {
		result.Artifacts[index] = projectdevloop.ArtifactReference{
			Path: artifact.Path, Digest: artifact.Digest,
		}
	}
	return result
}

func candidateSourceInvalid(err error) error {
	return fmt.Errorf(
		"%w: synchronized project sources are invalid: %v",
		project.ErrCandidateSourceInvalid,
		err,
	)
}
