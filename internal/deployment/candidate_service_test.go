package deployment

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCandidateServiceCreatesResumesAndBuildsCanonicalPreviewURL(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	repository := newCandidateMemoryRepository()
	service := newCandidateTestService(t, repository, now)
	digest := "sha256:" + strings.Repeat("a", 64)

	started, err := service.Start(context.Background(), StartCandidateRequest{
		ProjectID: "finance", OwnerID: "principal_1", ArtifactDigest: digest,
	})
	require.NoError(t, err)
	if started.Candidate.BaseGeneration != "deployment_7" {
		t.Fatalf("base generation = %q, want server-resolved deployment_7", started.Candidate.BaseGeneration)
	}
	resumed, err := service.Start(context.Background(), StartCandidateRequest{
		ProjectID: "finance", OwnerID: "principal_1", ArtifactDigest: digest,
	})
	require.NoError(t, err)
	if resumed.Candidate != started.Candidate || !resumed.Resumed || started.Resumed {
		t.Fatalf("started=%#v resumed=%#v", started, resumed)
	}
	wantURL := "https://prod.leapview.example/candidates/" + started.Candidate.ID
	if started.PreviewURL != wantURL || strings.Contains(started.PreviewURL, digest) ||
		strings.Contains(started.PreviewURL, "principal_1") {
		t.Fatalf("preview URL = %q, want %q without sensitive state", started.PreviewURL, wantURL)
	}

	_, err = service.Start(context.Background(), StartCandidateRequest{
		ProjectID: "finance", OwnerID: "principal_1",
		ArtifactDigest: "sha256:" + strings.Repeat("b", 64),
	})
	if !errors.Is(err, ErrCandidateConflict) {
		t.Fatalf("changed start error = %v, want explicit update conflict", err)
	}
}

func TestCandidateServiceIsolatesAutomationKeysAndCancelsByKey(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	repository := newCandidateMemoryRepository()
	service := newCandidateTestService(t, repository, now)
	digest := "sha256:" + strings.Repeat("a", 64)
	first, err := service.Start(t.Context(), StartCandidateRequest{
		ProjectID: "finance", OwnerID: "principal_1",
		ArtifactDigest: digest, Key: "github:pull/41",
	})
	require.NoError(t, err)
	second, err := service.Start(t.Context(), StartCandidateRequest{
		ProjectID: "finance", OwnerID: "principal_1",
		ArtifactDigest: digest, Key: "github:pull/42",
	})
	require.NoError(t, err)
	if first.Candidate.ID == second.Candidate.ID ||
		first.Candidate.Key == second.Candidate.Key {
		t.Fatalf("isolated candidates = %#v / %#v", first, second)
	}
	cancelled, err := service.CancelActive(
		t.Context(),
		"finance",
		"principal_1",
		"github:pull/41",
	)
	if err != nil || cancelled.ID != first.Candidate.ID ||
		cancelled.Status != CandidateCancelled {
		t.Fatalf("CancelActive() = %#v, %v", cancelled, err)
	}
	remaining, err := service.Get(t.Context(), CandidateScope{
		ProjectID: "finance", CandidateID: second.Candidate.ID,
		OwnerID: "principal_1",
	})
	if err != nil || remaining.Status != CandidatePreparing {
		t.Fatalf("remaining candidate = %#v, %v", remaining, err)
	}
}

func TestCandidateServiceConcealsForeignCandidatesAndUsesOptimisticReplacement(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	repository := newCandidateMemoryRepository()
	service := newCandidateTestService(t, repository, now)
	first := "sha256:" + strings.Repeat("a", 64)
	second := "sha256:" + strings.Repeat("b", 64)
	started, err := service.Start(context.Background(), StartCandidateRequest{
		ProjectID: "finance", OwnerID: "principal_1", ArtifactDigest: first,
	})
	require.NoError(t, err)
	for name, scope := range map[string]CandidateScope{
		"owner":   {ProjectID: "finance", CandidateID: started.Candidate.ID, OwnerID: "principal_2"},
		"project": {ProjectID: "marketing", CandidateID: started.Candidate.ID, OwnerID: "principal_1"},
		"target":  {ProjectID: "finance", CandidateID: started.Candidate.ID, OwnerID: "principal_1", TargetID: "lvinst_other"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := service.Get(context.Background(), scope); !errors.Is(err, ErrCandidateNotFound) {
				t.Fatalf("Get() error = %v, want concealed ErrCandidateNotFound", err)
			}
		})
	}

	updated, err := service.ReplaceArtifact(context.Background(), CandidateScope{
		ProjectID: "finance", CandidateID: started.Candidate.ID, OwnerID: "principal_1",
	}, first, second)
	require.NoError(t, err)
	if updated.ArtifactDigest != second || updated.Status != CandidatePreparing {
		t.Fatalf("updated = %#v", updated)
	}
	if _, err := service.ReplaceArtifact(context.Background(), CandidateScope{
		ProjectID: "finance", CandidateID: started.Candidate.ID, OwnerID: "principal_1",
	}, first, second); !errors.Is(err, ErrCandidateConflict) {
		t.Fatalf("stale replacement error = %v, want ErrCandidateConflict", err)
	}
}

func TestCandidateServiceEnforcesQuotaCancelExpiryAndRestartDurability(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	repository := newCandidateMemoryRepository()
	service := newCandidateTestService(t, repository, now)
	service.maxActivePerOwner = 1
	first, err := service.Start(context.Background(), StartCandidateRequest{
		ProjectID: "finance", OwnerID: "principal_1",
		ArtifactDigest: "sha256:" + strings.Repeat("a", 64),
	})
	require.NoError(t, err)
	if _, err := service.Start(context.Background(), StartCandidateRequest{
		ProjectID: "marketing", OwnerID: "principal_1",
		ArtifactDigest: "sha256:" + strings.Repeat("b", 64),
	}); !errors.Is(err, ErrCandidateQuota) {
		t.Fatalf("quota error = %v, want ErrCandidateQuota", err)
	}

	restarted := newCandidateTestService(t, repository, now.Add(10*time.Minute))
	resumed, err := restarted.Get(context.Background(), CandidateScope{
		ProjectID: "finance", CandidateID: first.Candidate.ID, OwnerID: "principal_1",
	})
	if err != nil || resumed.ID != first.Candidate.ID {
		t.Fatalf("restart Get() = %#v, %v", resumed, err)
	}
	cancelled, err := restarted.Cancel(context.Background(), CandidateScope{
		ProjectID: "finance", CandidateID: first.Candidate.ID, OwnerID: "principal_1",
	})
	if err != nil || cancelled.Status != CandidateCancelled {
		t.Fatalf("Cancel() = %#v, %v", cancelled, err)
	}
	replayed, err := restarted.Cancel(context.Background(), CandidateScope{
		ProjectID: "finance", CandidateID: first.Candidate.ID, OwnerID: "principal_1",
	})
	if err != nil || replayed != cancelled {
		t.Fatalf("replayed Cancel() = %#v, %v", replayed, err)
	}

	expiring := newCandidateTestService(t, repository, now)
	expiring.maxActivePerOwner = 2
	expiring.lifetime = time.Minute
	second, err := expiring.Start(context.Background(), StartCandidateRequest{
		ProjectID: "marketing", OwnerID: "principal_1",
		ArtifactDigest: "sha256:" + strings.Repeat("b", 64),
	})
	require.NoError(t, err)
	expiring.now = func() time.Time { return now.Add(2 * time.Minute) }
	if count, err := expiring.Reconcile(context.Background()); err != nil || count != 1 {
		t.Fatalf("Reconcile() = %d, %v", count, err)
	}
	expired, err := expiring.Get(context.Background(), CandidateScope{
		ProjectID: "marketing", CandidateID: second.Candidate.ID, OwnerID: "principal_1",
	})
	if err != nil || expired.Status != CandidateExpired {
		t.Fatalf("expired Get() = %#v, %v", expired, err)
	}
}

func TestCandidateServiceExpiresOwnedCandidateOnRead(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	repository := newCandidateMemoryRepository()
	service := newCandidateTestService(t, repository, now)
	service.lifetime = time.Minute
	started, err := service.Start(context.Background(), StartCandidateRequest{
		ProjectID: "finance", OwnerID: "principal_1",
		ArtifactDigest: "sha256:" + strings.Repeat("a", 64),
	})
	require.NoError(t, err)

	service.now = func() time.Time { return now.Add(2 * time.Minute) }
	expired, err := service.GetOwned(context.Background(), started.Candidate.ID, "principal_1")
	require.NoError(t, err)
	if expired.Status != CandidateExpired || expired.Revision != started.Candidate.Revision+1 {
		t.Fatalf("expired candidate = %#v", expired)
	}
	persisted, err := repository.CandidateByID(context.Background(), started.Candidate.ID)
	if err != nil || persisted != expired {
		t.Fatalf("persisted candidate = %#v, %v", persisted, err)
	}
}

func TestCandidateServiceAuditsLifecycleWithoutArtifactDigest(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	repository := newCandidateMemoryRepository()
	var events []CandidateEvent
	service := newCandidateTestService(t, repository, now)
	service.audit = func(_ context.Context, event CandidateEvent) error {
		events = append(events, event)
		return nil
	}
	digest := "sha256:" + strings.Repeat("a", 64)
	started, err := service.Start(context.Background(), StartCandidateRequest{
		ProjectID: "finance", OwnerID: "principal_1", ArtifactDigest: digest,
	})
	require.NoError(t, err)
	if _, err := service.Cancel(context.Background(), CandidateScope{
		ProjectID: "finance", CandidateID: started.Candidate.ID, OwnerID: "principal_1",
	}); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Action != "candidate.started" || events[1].Action != "candidate.cancelled" {
		t.Fatalf("events = %#v", events)
	}
	for _, event := range events {
		if strings.Contains(event.MetadataJSON, digest) {
			t.Fatalf("audit leaked artifact digest: %#v", event)
		}
	}
}

func newCandidateTestService(t *testing.T, repository CandidateRepository, now time.Time) *CandidateService {
	t.Helper()
	counter := 0
	service, err := NewCandidateService(repository, CandidateServiceConfig{
		TargetID: "lvinst_prod", CanonicalOrigin: "https://prod.leapview.example", Environment: "prod",
		Lifetime: time.Hour, MaxActivePerOwner: 4,
		Now: func() time.Time { return now },
		NewID: func() (string, error) {
			counter++
			return "cand_test_" + string(rune('0'+counter)), nil
		},
	})
	require.NoError(t, err)
	return service
}

type candidateMemoryRepository struct {
	candidates      map[string]Candidate
	baseGenerations map[string]string
}

func newCandidateMemoryRepository() *candidateMemoryRepository {
	return &candidateMemoryRepository{
		candidates: map[string]Candidate{},
		baseGenerations: map[string]string{
			"finance":   "deployment_7",
			"marketing": "deployment_8",
		},
	}
}

func (repository *candidateMemoryRepository) ActiveCandidateBaseGeneration(_ context.Context, projectID, _ string) (string, error) {
	if generation := repository.baseGenerations[projectID]; generation != "" {
		return generation, nil
	}
	return CandidateBaseGenerationEmpty, nil
}

func (repository *candidateMemoryRepository) StartCandidate(_ context.Context, candidate Candidate, maxActivePerOwner int) (Candidate, bool, error) {
	active := 0
	for _, existing := range repository.candidates {
		if existing.OwnerID == candidate.OwnerID && !existing.Terminal() {
			active++
		}
		if existing.OwnerID == candidate.OwnerID && existing.ProjectID == candidate.ProjectID &&
			existing.TargetID == candidate.TargetID && existing.Key == candidate.Key &&
			!existing.Terminal() {
			if existing.BaseGeneration == candidate.BaseGeneration && existing.ArtifactDigest == candidate.ArtifactDigest {
				return existing, true, nil
			}
			return Candidate{}, false, ErrCandidateConflict
		}
	}
	if active >= maxActivePerOwner {
		return Candidate{}, false, ErrCandidateQuota
	}
	repository.candidates[candidate.ID] = candidate
	return candidate, false, nil
}

func (repository *candidateMemoryRepository) ActiveCandidate(
	_ context.Context,
	targetID,
	projectID,
	ownerID,
	key string,
) (Candidate, error) {
	for _, candidate := range repository.candidates {
		if candidate.TargetID == targetID && candidate.ProjectID == projectID &&
			candidate.OwnerID == ownerID && candidate.Key == key &&
			!candidate.Terminal() {
			return candidate, nil
		}
	}
	return Candidate{}, ErrCandidateNotFound
}

func (repository *candidateMemoryRepository) CandidateByID(_ context.Context, id string) (Candidate, error) {
	candidate, ok := repository.candidates[id]
	if !ok {
		return Candidate{}, ErrCandidateNotFound
	}
	return candidate, nil
}

func (repository *candidateMemoryRepository) SaveCandidate(_ context.Context, candidate Candidate, expectedRevision int64) (Candidate, error) {
	existing, ok := repository.candidates[candidate.ID]
	if !ok {
		return Candidate{}, ErrCandidateNotFound
	}
	if existing.Revision != expectedRevision {
		return Candidate{}, ErrCandidateConflict
	}
	repository.candidates[candidate.ID] = candidate
	return candidate, nil
}

func (repository *candidateMemoryRepository) ExpireCandidates(_ context.Context, targetID string, now time.Time) (int64, error) {
	var count int64
	for id, candidate := range repository.candidates {
		if candidate.TargetID != targetID {
			continue
		}
		expired, changed, err := candidate.Expire(now)
		if err != nil {
			return count, err
		}
		if changed {
			repository.candidates[id] = expired
			count++
		}
	}
	return count, nil
}
