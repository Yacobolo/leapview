package runtimehost

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	servingstate "github.com/flidai/leapview/internal/servingstate"
)

var (
	ErrCandidateRuntimeInvalid      = errors.New("candidate runtime invalid")
	ErrCandidateRuntimeNotFound     = errors.New("candidate runtime not found")
	ErrCandidateRuntimeIncompatible = errors.New("candidate runtime incompatible")
	ErrCandidateRuntimeConflict     = errors.New("candidate runtime conflict")
	ErrCandidateRuntimeExpired      = errors.New("candidate runtime expired")
	ErrCandidateRuntimeClosed       = errors.New("candidate runtime registry closed")
)

// CandidateBindingVersion is the non-secret identity of one validated target
// connection generation used to prepare a candidate runtime.
type CandidateBindingVersion struct {
	BindingID       string
	Revision        int64
	ProviderVersion string
}

// CandidateCompatibility describes every runtime-wide boundary that must
// remain equal before a private candidate generation can be leased.
//
// Query-specific semantic and effective-policy fingerprints remain part of
// query/result cache keys. AuthorizationFingerprint is the effective
// principal/policy boundary for acquiring this runtime generation.
type CandidateCompatibility struct {
	ArtifactDigest           string
	DataRevision             string
	RuntimeVersion           string
	AuthorizationFingerprint string
	Bindings                 []CandidateBindingVersion
}

type CandidateRegistration struct {
	CandidateID   string
	OwnerID       string
	WorkspaceID   servingstate.WorkspaceID
	ExpiresAt     time.Time
	Compatibility CandidateCompatibility
}

type CandidateLeaseRequest struct {
	CandidateID   string
	OwnerID       string
	WorkspaceID   servingstate.WorkspaceID
	Compatibility CandidateCompatibility
}

type candidateRuntimeKey struct {
	candidateID string
	workspaceID servingstate.WorkspaceID
}

type candidateGeneration struct {
	key           candidateRuntimeKey
	ownerID       string
	expiresAt     time.Time
	compatibility [sha256.Size]byte
	manager       *Manager
	managed       *managedRuntime
	refs          int
	closing       bool
}

type candidateRuntimeRegistry struct {
	mu      sync.Mutex
	now     func() time.Time
	current map[candidateRuntimeKey]*candidateGeneration
	retired map[*candidateGeneration]struct{}
	closed  bool
}

func newCandidateRuntimeRegistry(now func() time.Time) *candidateRuntimeRegistry {
	if now == nil {
		now = time.Now
	}
	return &candidateRuntimeRegistry{
		now: now, current: map[candidateRuntimeKey]*candidateGeneration{},
		retired: map[*candidateGeneration]struct{}{},
	}
}

// RegisterPreparedCandidate transfers an isolated prepared runtime into the
// private candidate registry without publishing it as an active generation.
func (r *Registry) RegisterPreparedCandidate(
	registration CandidateRegistration,
	candidate servingstate.PreparedRuntime,
) error {
	if r == nil || r.candidates == nil {
		return ErrCandidateRuntimeClosed
	}
	normalized, fingerprint, err := normalizeCandidateRegistration(registration, r.candidates.now())
	if err != nil {
		return err
	}
	prepared, ok := candidate.(*RegistryPrepared)
	if !ok || prepared == nil || prepared.registry != r {
		return fmt.Errorf("%w: prepared runtime belongs to a different host", ErrCandidateRuntimeInvalid)
	}
	if prepared.workspaceID != normalized.WorkspaceID {
		return fmt.Errorf(
			"%w: prepared workspace %q does not match registration workspace %q",
			ErrCandidateRuntimeInvalid, prepared.workspaceID, normalized.WorkspaceID,
		)
	}
	sealed, err := r.sealRegistryPrepared(prepared)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCandidateRuntimeInvalid, err)
	}
	managed, err := sealed.consumeCandidate()
	if err != nil {
		return err
	}
	generation := &candidateGeneration{
		key: candidateRuntimeKey{
			candidateID: normalized.CandidateID, workspaceID: normalized.WorkspaceID,
		},
		ownerID: normalized.OwnerID, expiresAt: normalized.ExpiresAt,
		compatibility: fingerprint, manager: sealed.manager, managed: managed,
	}
	retired, err := r.candidates.register(generation)
	if err != nil {
		generation.closing = true
		generation.managed.closing = true
		generation.manager.cleanupRetired(generation.managed)
		return err
	}
	r.cleanupCandidateGeneration(retired)
	return nil
}

func (r *Registry) AcquireCandidate(ctx context.Context, request CandidateLeaseRequest) (Lease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r == nil || r.candidates == nil {
		return nil, ErrCandidateRuntimeClosed
	}
	normalized, fingerprint, err := normalizeCandidateLeaseRequest(request)
	if err != nil {
		return nil, err
	}
	generation, retired, err := r.candidates.acquire(
		normalized.CandidateID,
		normalized.OwnerID,
		normalized.WorkspaceID,
		fingerprint,
	)
	r.cleanupCandidateGeneration(retired)
	if err != nil {
		return nil, err
	}
	return &candidateRuntimeLease{registry: r, generation: generation}, nil
}

// RetireCandidate stops all new acquisitions for a candidate while allowing
// existing query leases to drain safely.
func (r *Registry) RetireCandidate(candidateID string) int {
	if r == nil || r.candidates == nil {
		return 0
	}
	retired, count := r.candidates.retireCandidate(strings.TrimSpace(candidateID))
	for _, generation := range retired {
		r.cleanupCandidateGeneration(generation)
	}
	return count
}

func (r *Registry) ReapExpiredCandidates(now time.Time) int {
	if r == nil || r.candidates == nil {
		return 0
	}
	retired, count := r.candidates.reapExpired(now.UTC())
	for _, generation := range retired {
		r.cleanupCandidateGeneration(generation)
	}
	return count
}

func (r *Registry) cleanupCandidateGeneration(generation *candidateGeneration) {
	if generation == nil || generation.manager == nil || generation.managed == nil {
		return
	}
	generation.manager.cleanupRetired(generation.managed)
}

func (r *candidateRuntimeRegistry) register(
	generation *candidateGeneration,
) (*candidateGeneration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, ErrCandidateRuntimeClosed
	}
	current := r.current[generation.key]
	if current != nil && current.ownerID != generation.ownerID {
		return nil, ErrCandidateRuntimeConflict
	}
	r.current[generation.key] = generation
	return r.retireLocked(current), nil
}

func (r *candidateRuntimeRegistry) acquire(
	candidateID string,
	ownerID string,
	workspaceID servingstate.WorkspaceID,
	compatibility [sha256.Size]byte,
) (generation *candidateGeneration, retired *candidateGeneration, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, nil, ErrCandidateRuntimeClosed
	}
	key := candidateRuntimeKey{candidateID: candidateID, workspaceID: workspaceID}
	current := r.current[key]
	if current == nil || current.ownerID != ownerID {
		return nil, nil, ErrCandidateRuntimeNotFound
	}
	if !r.now().UTC().Before(current.expiresAt) {
		delete(r.current, key)
		return nil, r.retireLocked(current), ErrCandidateRuntimeExpired
	}
	if current.compatibility != compatibility {
		return nil, nil, ErrCandidateRuntimeIncompatible
	}
	current.refs++
	return current, nil, nil
}

func (r *candidateRuntimeRegistry) retireCandidate(
	candidateID string,
) (drained []*candidateGeneration, count int) {
	if candidateID == "" {
		return nil, 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, generation := range r.current {
		if key.candidateID != candidateID {
			continue
		}
		delete(r.current, key)
		count++
		if generation := r.retireLocked(generation); generation != nil {
			drained = append(drained, generation)
		}
	}
	return drained, count
}

func (r *candidateRuntimeRegistry) reapExpired(
	now time.Time,
) (drained []*candidateGeneration, count int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, generation := range r.current {
		if now.Before(generation.expiresAt) {
			continue
		}
		delete(r.current, key)
		count++
		if generation := r.retireLocked(generation); generation != nil {
			drained = append(drained, generation)
		}
	}
	return drained, count
}

func (r *candidateRuntimeRegistry) retireLocked(
	generation *candidateGeneration,
) *candidateGeneration {
	if generation == nil || generation.closing {
		return nil
	}
	generation.closing = true
	generation.managed.closing = true
	if generation.refs > 0 {
		r.retired[generation] = struct{}{}
		return nil
	}
	return generation
}

func (r *candidateRuntimeRegistry) release(generation *candidateGeneration) *candidateGeneration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if generation == nil || generation.refs == 0 {
		return nil
	}
	generation.refs--
	if generation.refs != 0 || !generation.closing {
		return nil
	}
	delete(r.retired, generation)
	return generation
}

func (r *candidateRuntimeRegistry) close() []*candidateGeneration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	var drained []*candidateGeneration
	for key, generation := range r.current {
		delete(r.current, key)
		if closed := r.retireLocked(generation); closed != nil {
			drained = append(drained, closed)
		}
	}
	return drained
}

func (r *candidateRuntimeRegistry) leasedSnapshots() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshots := map[int64]struct{}{}
	for _, generation := range r.current {
		if generation.managed.snapshotLease != nil && generation.managed.snapshotID > 0 {
			snapshots[generation.managed.snapshotID] = struct{}{}
		}
	}
	for generation := range r.retired {
		if generation.managed.snapshotLease != nil && generation.managed.snapshotID > 0 {
			snapshots[generation.managed.snapshotID] = struct{}{}
		}
	}
	return snapshotKeys(snapshots)
}

type candidateRuntimeLease struct {
	registry   *Registry
	generation *candidateGeneration
	once       sync.Once
}

func (l *candidateRuntimeLease) Runtime() Runtime {
	if l == nil || l.generation == nil || l.generation.managed == nil {
		return nil
	}
	return l.generation.managed.runtime
}

func (l *candidateRuntimeLease) ServingStateID() servingstate.ID {
	if l == nil || l.generation == nil || l.generation.managed == nil {
		return ""
	}
	return l.generation.managed.servingStateID
}

func (l *candidateRuntimeLease) DuckLakeSnapshotID() int64 {
	if l == nil || l.generation == nil || l.generation.managed == nil {
		return 0
	}
	return l.generation.managed.snapshotID
}

func (l *candidateRuntimeLease) Release() {
	if l == nil || l.registry == nil || l.registry.candidates == nil || l.generation == nil {
		return
	}
	l.once.Do(func() {
		l.registry.cleanupCandidateGeneration(l.registry.candidates.release(l.generation))
	})
}

func normalizeCandidateRegistration(
	registration CandidateRegistration,
	now time.Time,
) (CandidateRegistration, [sha256.Size]byte, error) {
	registration.CandidateID = strings.TrimSpace(registration.CandidateID)
	registration.OwnerID = strings.TrimSpace(registration.OwnerID)
	registration.WorkspaceID = servingstate.WorkspaceID(strings.TrimSpace(string(registration.WorkspaceID)))
	registration.ExpiresAt = registration.ExpiresAt.UTC()
	if registration.CandidateID == "" || registration.OwnerID == "" || registration.WorkspaceID == "" {
		return CandidateRegistration{}, [sha256.Size]byte{}, fmt.Errorf(
			"%w: candidate, owner, and workspace are required",
			ErrCandidateRuntimeInvalid,
		)
	}
	if registration.ExpiresAt.IsZero() || !registration.ExpiresAt.After(now.UTC()) {
		return CandidateRegistration{}, [sha256.Size]byte{}, fmt.Errorf(
			"%w: expiry must be in the future",
			ErrCandidateRuntimeInvalid,
		)
	}
	compatibility, fingerprint, err := normalizeCandidateCompatibility(registration.Compatibility)
	if err != nil {
		return CandidateRegistration{}, [sha256.Size]byte{}, err
	}
	registration.Compatibility = compatibility
	return registration, fingerprint, nil
}

func normalizeCandidateLeaseRequest(
	request CandidateLeaseRequest,
) (CandidateLeaseRequest, [sha256.Size]byte, error) {
	request.CandidateID = strings.TrimSpace(request.CandidateID)
	request.OwnerID = strings.TrimSpace(request.OwnerID)
	request.WorkspaceID = servingstate.WorkspaceID(strings.TrimSpace(string(request.WorkspaceID)))
	if request.CandidateID == "" || request.OwnerID == "" || request.WorkspaceID == "" {
		return CandidateLeaseRequest{}, [sha256.Size]byte{}, fmt.Errorf(
			"%w: candidate, owner, and workspace are required",
			ErrCandidateRuntimeInvalid,
		)
	}
	compatibility, fingerprint, err := normalizeCandidateCompatibility(request.Compatibility)
	if err != nil {
		return CandidateLeaseRequest{}, [sha256.Size]byte{}, err
	}
	request.Compatibility = compatibility
	return request, fingerprint, nil
}

func normalizeCandidateCompatibility(
	compatibility CandidateCompatibility,
) (CandidateCompatibility, [sha256.Size]byte, error) {
	compatibility.ArtifactDigest = strings.TrimSpace(compatibility.ArtifactDigest)
	compatibility.DataRevision = strings.TrimSpace(compatibility.DataRevision)
	compatibility.RuntimeVersion = strings.TrimSpace(compatibility.RuntimeVersion)
	compatibility.AuthorizationFingerprint = strings.TrimSpace(compatibility.AuthorizationFingerprint)
	if compatibility.ArtifactDigest == "" || compatibility.DataRevision == "" ||
		compatibility.RuntimeVersion == "" || compatibility.AuthorizationFingerprint == "" {
		return CandidateCompatibility{}, [sha256.Size]byte{}, fmt.Errorf(
			"%w: artifact, data, runtime, and authorization fingerprints are required",
			ErrCandidateRuntimeInvalid,
		)
	}
	normalizedBindings := append([]CandidateBindingVersion(nil), compatibility.Bindings...)
	for index := range normalizedBindings {
		normalizedBindings[index].BindingID = strings.TrimSpace(normalizedBindings[index].BindingID)
		normalizedBindings[index].ProviderVersion = strings.TrimSpace(normalizedBindings[index].ProviderVersion)
		if normalizedBindings[index].BindingID == "" || normalizedBindings[index].Revision < 1 ||
			normalizedBindings[index].ProviderVersion == "" {
			return CandidateCompatibility{}, [sha256.Size]byte{}, fmt.Errorf(
				"%w: binding identity, positive revision, and provider version are required",
				ErrCandidateRuntimeInvalid,
			)
		}
	}
	sort.Slice(normalizedBindings, func(i, j int) bool {
		return normalizedBindings[i].BindingID < normalizedBindings[j].BindingID
	})
	for index := 1; index < len(normalizedBindings); index++ {
		if normalizedBindings[index-1].BindingID == normalizedBindings[index].BindingID {
			return CandidateCompatibility{}, [sha256.Size]byte{}, fmt.Errorf(
				"%w: duplicate binding %q",
				ErrCandidateRuntimeInvalid,
				normalizedBindings[index].BindingID,
			)
		}
	}
	compatibility.Bindings = normalizedBindings
	encoded, err := json.Marshal(compatibility)
	if err != nil {
		return CandidateCompatibility{}, [sha256.Size]byte{}, fmt.Errorf(
			"%w: encode compatibility: %v",
			ErrCandidateRuntimeInvalid,
			err,
		)
	}
	return compatibility, sha256.Sum256(encoded), nil
}
