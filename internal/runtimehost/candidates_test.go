package runtimehost

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	servingstate "github.com/flidai/leapview/internal/servingstate"
)

func TestCandidateRuntimeReplacementIsPrivateAndDrainsLeasedGeneration(t *testing.T) {
	now := time.Date(2026, 7, 29, 17, 0, 0, 0, time.UTC)
	repo := newFakeRegistryRepo()
	repo.active["sales/prod"] = registryDeploymentArtifact{
		deployment: servingstate.State{
			ID: "active_sales", WorkspaceID: "sales", Environment: "prod",
			Status: servingstate.StatusActive, DuckLakeSnapshotID: 11,
		},
		artifact: servingstate.Artifact{
			ServingStateID: "active_sales", WorkspaceID: "sales", Environment: "prod", Digest: "active",
		},
	}
	addCandidateServingState(repo, "candidate_sales_1", "sales", "candidate-1", 21)
	addCandidateServingState(repo, "candidate_sales_2", "sales", "candidate-2", 22)
	factory := &recordingRegistryFactory{}
	registry := NewRegistryWithFactory(RegistryOptions{
		Repo: repo, WorkspaceIDs: []servingstate.WorkspaceID{"sales"}, Environment: "prod",
		Factory: factory, Now: func() time.Time { return now },
	})
	if err := registry.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = registry.Close() })

	registerCandidateRuntime(t, registry, CandidateRegistration{
		CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
		ExpiresAt: now.Add(time.Hour), Compatibility: candidateCompatibility("one"),
	}, "candidate_sales_1")
	oldLease, err := registry.AcquireCandidate(t.Context(), CandidateLeaseRequest{
		CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
		Compatibility: candidateCompatibility("one"),
	})
	if err != nil {
		t.Fatal(err)
	}
	oldRuntime := oldLease.Runtime().(*recordingRuntime)

	registerCandidateRuntime(t, registry, CandidateRegistration{
		CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
		ExpiresAt: now.Add(time.Hour), Compatibility: candidateCompatibility("two"),
	}, "candidate_sales_2")
	if oldRuntime.closed {
		t.Fatal("replaced candidate runtime closed while a lease was active")
	}
	newLease, err := registry.AcquireCandidate(t.Context(), CandidateLeaseRequest{
		CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
		Compatibility: candidateCompatibility("two"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if newLease.ServingStateID() != "candidate_sales_2" || newLease.DuckLakeSnapshotID() != 22 {
		t.Fatalf("new candidate lease = (%s, %d)", newLease.ServingStateID(), newLease.DuckLakeSnapshotID())
	}
	newLease.Release()

	active, err := registry.AcquireForWorkspace(t.Context(), "sales")
	if err != nil {
		t.Fatal(err)
	}
	if active.ServingStateID() != "active_sales" || active.DuckLakeSnapshotID() != 11 {
		t.Fatalf("candidate replacement changed active runtime = (%s, %d)", active.ServingStateID(), active.DuckLakeSnapshotID())
	}
	active.Release()

	oldLease.Release()
	if !oldRuntime.closed {
		t.Fatal("replaced candidate runtime remained open after its final lease")
	}
}

func TestCandidateRuntimeLeaseFailsClosedAcrossOwnershipAndCompatibilityBoundaries(t *testing.T) {
	now := time.Date(2026, 7, 29, 17, 0, 0, 0, time.UTC)
	registry := candidateTestRegistry(t, func() time.Time { return now })
	registerCandidateRuntime(t, registry, CandidateRegistration{
		CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
		ExpiresAt: now.Add(time.Hour), Compatibility: candidateCompatibility("one"),
	}, "candidate_sales_1")

	for name, request := range map[string]CandidateLeaseRequest{
		"owner": {
			CandidateID: "cand_1", OwnerID: "author_2", WorkspaceID: "sales",
			Compatibility: candidateCompatibility("one"),
		},
		"workspace": {
			CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "operations",
			Compatibility: candidateCompatibility("one"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := registry.AcquireCandidate(t.Context(), request); !errors.Is(err, ErrCandidateRuntimeNotFound) {
				t.Fatalf("AcquireCandidate() error = %v, want concealed not found", err)
			}
		})
	}
	if _, err := registry.AcquireCandidate(t.Context(), CandidateLeaseRequest{
		CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
		Compatibility: candidateCompatibility("two"),
	}); !errors.Is(err, ErrCandidateRuntimeIncompatible) {
		t.Fatalf("AcquireCandidate() error = %v, want incompatible", err)
	}
}

func TestCandidateRuntimeExpiryRejectsNewLeasesAndDrainsExistingLease(t *testing.T) {
	now := time.Date(2026, 7, 29, 17, 0, 0, 0, time.UTC)
	registry := candidateTestRegistry(t, func() time.Time { return now })
	registerCandidateRuntime(t, registry, CandidateRegistration{
		CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
		ExpiresAt: now.Add(time.Minute), Compatibility: candidateCompatibility("one"),
	}, "candidate_sales_1")
	request := CandidateLeaseRequest{
		CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
		Compatibility: candidateCompatibility("one"),
	}
	lease, err := registry.AcquireCandidate(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	runtime := lease.Runtime().(*recordingRuntime)

	now = now.Add(2 * time.Minute)
	if _, err := registry.AcquireCandidate(t.Context(), request); !errors.Is(err, ErrCandidateRuntimeExpired) {
		t.Fatalf("expired AcquireCandidate() error = %v", err)
	}
	if runtime.closed {
		t.Fatal("expired candidate runtime closed while an existing lease was active")
	}
	lease.Release()
	if !runtime.closed {
		t.Fatal("expired candidate runtime remained open after its final lease")
	}
	if removed := registry.ReapExpiredCandidates(now); removed != 0 {
		t.Fatalf("reaped candidates = %d, want already retired generation", removed)
	}
}

func TestCandidateRuntimeRetirementIsSafeWithConcurrentLeaseRelease(t *testing.T) {
	now := time.Date(2026, 7, 29, 17, 0, 0, 0, time.UTC)
	registry := candidateTestRegistry(t, func() time.Time { return now })
	registerCandidateRuntime(t, registry, CandidateRegistration{
		CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
		ExpiresAt: now.Add(time.Hour), Compatibility: candidateCompatibility("one"),
	}, "candidate_sales_1")
	request := CandidateLeaseRequest{
		CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
		Compatibility: candidateCompatibility("one"),
	}
	leases := make([]Lease, 32)
	for index := range leases {
		lease, err := registry.AcquireCandidate(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		leases[index] = lease
	}
	runtime := leases[0].Runtime().(*recordingRuntime)

	var wait sync.WaitGroup
	wait.Add(len(leases) + 1)
	for _, lease := range leases {
		go func(lease Lease) {
			defer wait.Done()
			lease.Release()
		}(lease)
	}
	go func() {
		defer wait.Done()
		registry.RetireCandidate("cand_1")
	}()
	wait.Wait()

	if !runtime.closed {
		t.Fatal("retired candidate runtime remained open after concurrent releases")
	}
	if _, err := registry.AcquireCandidate(context.Background(), request); !errors.Is(err, ErrCandidateRuntimeNotFound) {
		t.Fatalf("AcquireCandidate() after retirement error = %v", err)
	}
}

func TestCandidateRuntimeOwnsExternalDependenciesUntilGenerationDrains(t *testing.T) {
	now := time.Date(2026, 7, 29, 17, 0, 0, 0, time.UTC)
	registry := candidateTestRegistry(t, func() time.Time { return now })
	lifetime := &candidateTestLifetime{}
	registration := CandidateRegistration{
		CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
		ExpiresAt: now.Add(time.Hour), Compatibility: candidateCompatibility("one"),
	}
	if err := registry.PrepareAndRegisterCandidate(
		t.Context(),
		CandidatePreparation{
			Registration: registration, ServingStateID: "candidate_sales_1",
			Lifetime: lifetime,
		},
	); err != nil {
		t.Fatal(err)
	}
	lease, err := registry.AcquireCandidate(t.Context(), CandidateLeaseRequest{
		CandidateID: registration.CandidateID,
		OwnerID:     registration.OwnerID, WorkspaceID: registration.WorkspaceID,
		Compatibility: registration.Compatibility,
	})
	if err != nil {
		t.Fatal(err)
	}
	registry.RetireCandidate(registration.CandidateID)
	if lifetime.closes != 0 {
		t.Fatal("candidate dependency closed while a runtime lease was active")
	}
	lease.Release()
	if lifetime.closes != 1 {
		t.Fatalf("candidate dependency closes = %d, want 1 after drain", lifetime.closes)
	}
}

func TestCandidateRuntimeRejectsRegistrationUnderDifferentCompatibility(t *testing.T) {
	now := time.Date(2026, 7, 29, 17, 0, 0, 0, time.UTC)
	registry := candidateTestRegistry(t, func() time.Time { return now })
	prepared, err := registry.PrepareCandidate(t.Context(), CandidatePreparation{
		Registration: CandidateRegistration{
			CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
			ExpiresAt: now.Add(time.Hour), Compatibility: candidateCompatibility("one"),
		},
		ServingStateID: "candidate_sales_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()

	err = registry.RegisterPreparedCandidate(CandidateRegistration{
		CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
		ExpiresAt: now.Add(time.Hour), Compatibility: candidateCompatibility("two"),
	}, prepared)
	if !errors.Is(err, ErrCandidateRuntimeIncompatible) {
		t.Fatalf("RegisterPreparedCandidate() error = %v, want incompatible", err)
	}
}

func TestCandidateRuntimeSetReplacesEveryWorkspaceAsOneGeneration(t *testing.T) {
	now := time.Date(2026, 7, 29, 17, 0, 0, 0, time.UTC)
	repo := newFakeRegistryRepo()
	for _, state := range []struct {
		id        servingstate.ID
		workspace servingstate.WorkspaceID
		snapshot  int64
	}{
		{id: "sales_1", workspace: "sales", snapshot: 11},
		{id: "ops_1", workspace: "operations", snapshot: 12},
		{id: "sales_2", workspace: "sales", snapshot: 21},
		{id: "ops_2", workspace: "operations", snapshot: 22},
	} {
		addCandidateServingState(repo, state.id, state.workspace, string(state.id), state.snapshot)
	}
	registry := NewRegistryWithFactory(RegistryOptions{
		Repo: repo, Environment: "prod", Factory: &recordingRegistryFactory{},
		Now: func() time.Time { return now },
	})
	t.Cleanup(func() { _ = registry.Close() })
	prepareSet := func(suffix string) []CandidatePreparation {
		return []CandidatePreparation{
			{
				Registration: CandidateRegistration{
					CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
					ExpiresAt: now.Add(time.Hour), Compatibility: candidateCompatibility("sales-" + suffix),
				},
				ServingStateID: "sales_" + suffix,
			},
			{
				Registration: CandidateRegistration{
					CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "operations",
					ExpiresAt: now.Add(time.Hour), Compatibility: candidateCompatibility("ops-" + suffix),
				},
				ServingStateID: "ops_" + suffix,
			},
		}
	}
	if err := registry.PrepareAndRegisterCandidateSet(t.Context(), prepareSet("1")); err != nil {
		t.Fatal(err)
	}
	old, err := registry.AcquireCandidate(t.Context(), CandidateLeaseRequest{
		CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
		Compatibility: candidateCompatibility("sales-1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	oldRuntime := old.Runtime().(*recordingRuntime)

	if err := registry.PrepareAndRegisterCandidateSet(t.Context(), prepareSet("2")); err != nil {
		t.Fatal(err)
	}
	for workspace, compatibility := range map[servingstate.WorkspaceID]CandidateCompatibility{
		"sales":      candidateCompatibility("sales-2"),
		"operations": candidateCompatibility("ops-2"),
	} {
		lease, err := registry.AcquireCandidate(t.Context(), CandidateLeaseRequest{
			CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: workspace,
			Compatibility: compatibility,
		})
		if err != nil {
			t.Fatalf("acquire %s: %v", workspace, err)
		}
		if lease.DuckLakeSnapshotID() < 20 {
			t.Fatalf("%s candidate retained old snapshot %d", workspace, lease.DuckLakeSnapshotID())
		}
		lease.Release()
	}
	if oldRuntime.closed {
		t.Fatal("old workspace runtime closed before its outstanding lease drained")
	}
	old.Release()
	if !oldRuntime.closed {
		t.Fatal("old workspace runtime remained open after set replacement drained")
	}
}

func TestCandidateRuntimeSetClosesEverySuppliedLifetimeWhenPreparationFails(t *testing.T) {
	now := time.Date(2026, 7, 29, 17, 0, 0, 0, time.UTC)
	repo := newFakeRegistryRepo()
	addCandidateServingState(repo, "sales_1", "sales", "sales", 11)
	registry := NewRegistryWithFactory(RegistryOptions{
		Repo: repo, Environment: "prod", Factory: &recordingRegistryFactory{},
		Now: func() time.Time { return now },
	})
	t.Cleanup(func() { _ = registry.Close() })
	lifetimes := []*candidateTestLifetime{{}, {}, {}}
	inputs := []CandidatePreparation{
		{
			Registration: CandidateRegistration{
				CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
				ExpiresAt: now.Add(time.Hour), Compatibility: candidateCompatibility("sales"),
			},
			ServingStateID: "sales_1", Lifetime: lifetimes[0],
		},
		{
			Registration: CandidateRegistration{
				CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "operations",
				ExpiresAt: now.Add(time.Hour), Compatibility: candidateCompatibility("operations"),
			},
			ServingStateID: "missing", Lifetime: lifetimes[1],
		},
		{
			Registration: CandidateRegistration{
				CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "finance",
				ExpiresAt: now.Add(time.Hour), Compatibility: candidateCompatibility("finance"),
			},
			ServingStateID: "unreached", Lifetime: lifetimes[2],
		},
	}

	if err := registry.PrepareAndRegisterCandidateSet(t.Context(), inputs); err == nil {
		t.Fatal("PrepareAndRegisterCandidateSet() error = nil, want preparation failure")
	}
	for index, lifetime := range lifetimes {
		if lifetime.closes != 1 {
			t.Fatalf("lifetime %d closes = %d, want 1", index, lifetime.closes)
		}
	}
}

func TestCandidateRuntimeDataModeFailsClosedAgainstServingSnapshotAndBindings(t *testing.T) {
	now := time.Date(2026, 7, 29, 17, 0, 0, 0, time.UTC)
	repo := newFakeRegistryRepo()
	addCandidateServingState(repo, "with_snapshot", "sales", "snapshot", 11)
	addCandidateServingState(repo, "without_snapshot", "sales", "refresh", 0)
	registry := NewRegistryWithFactory(RegistryOptions{
		Repo: repo, Environment: "prod", Factory: &recordingRegistryFactory{},
		Now: func() time.Time { return now },
	})
	t.Cleanup(func() { _ = registry.Close() })
	for name, input := range map[string]CandidatePreparation{
		"reuse_without_snapshot": {
			Registration: CandidateRegistration{
				CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
				ExpiresAt: now.Add(time.Hour),
				Compatibility: CandidateCompatibility{
					ArtifactDigest: "artifact", DataRevision: "data",
					DataMode:       CandidateDataReuseSnapshot,
					RuntimeVersion: "runtime", AuthorizationFingerprint: "policy",
				},
			},
			ServingStateID: "without_snapshot",
		},
		"refresh_with_snapshot": {
			Registration: CandidateRegistration{
				CandidateID: "cand_1", OwnerID: "author_1", WorkspaceID: "sales",
				ExpiresAt: now.Add(time.Hour),
				Compatibility: CandidateCompatibility{
					ArtifactDigest: "artifact", DataRevision: "data",
					DataMode:       CandidateDataRefreshSources,
					RuntimeVersion: "runtime", AuthorizationFingerprint: "policy",
					Bindings: []CandidateBindingVersion{{
						BindingID: "warehouse", Revision: 1, ProviderVersion: "provider",
					}},
				},
			},
			ServingStateID: "with_snapshot",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := registry.PrepareCandidate(t.Context(), input); !errors.Is(
				err,
				ErrCandidateRuntimeIncompatible,
			) {
				t.Fatalf("PrepareCandidate() error = %v, want incompatible", err)
			}
		})
	}
}

func candidateTestRegistry(t *testing.T, now func() time.Time) *Registry {
	t.Helper()
	repo := newFakeRegistryRepo()
	addCandidateServingState(repo, "candidate_sales_1", "sales", "candidate-1", 21)
	registry := NewRegistryWithFactory(RegistryOptions{
		Repo: repo, Environment: "prod", Factory: &recordingRegistryFactory{},
		Now: now,
	})
	t.Cleanup(func() { _ = registry.Close() })
	return registry
}

func addCandidateServingState(
	repo *fakeRegistryRepo,
	id servingstate.ID,
	workspace servingstate.WorkspaceID,
	digest string,
	snapshotID int64,
) {
	repo.deployments[id] = servingstate.State{
		ID: id, WorkspaceID: workspace, Environment: "prod",
		Status: servingstate.StatusValidated, DuckLakeSnapshotID: snapshotID,
	}
	repo.artifacts[id] = servingstate.Artifact{
		ServingStateID: id, WorkspaceID: workspace, Environment: "prod", Digest: digest,
	}
}

func registerCandidateRuntime(
	t *testing.T,
	registry *Registry,
	registration CandidateRegistration,
	servingStateID string,
) {
	t.Helper()
	if err := registry.PrepareAndRegisterCandidate(t.Context(), CandidatePreparation{
		Registration: registration, ServingStateID: servingStateID,
	}); err != nil {
		t.Fatal(err)
	}
}

func candidateCompatibility(suffix string) CandidateCompatibility {
	return CandidateCompatibility{
		ArtifactDigest:           "artifact-" + suffix,
		DataRevision:             "data-" + suffix,
		DataMode:                 CandidateDataReuseSnapshot,
		RuntimeVersion:           "runtime-v1",
		AuthorizationFingerprint: "policy-" + suffix,
	}
}

type candidateTestLifetime struct {
	closes int
}

func (lifetime *candidateTestLifetime) Close() error {
	lifetime.closes++
	return nil
}
