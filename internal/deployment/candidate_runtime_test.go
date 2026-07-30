package deployment

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/flidai/leapview/internal/runtimehost"
)

func TestCandidateRuntimeServicePreparesCredentialFreeAndBoundWorkspacesTogether(t *testing.T) {
	now := time.Date(2026, 7, 29, 18, 0, 0, 0, time.UTC)
	candidate := candidateRuntimeTestCandidate(t, now)
	connections := &candidateRuntimeConnections{}
	host := &candidateRuntimeHost{}
	service, err := NewCandidateRuntimeService(CandidateRuntimeServiceConfig{
		Connections: connections, Runtime: host, RuntimeVersion: "leapview:test",
	})
	if err != nil {
		t.Fatal(err)
	}

	receipt, err := service.Prepare(t.Context(), CandidateRuntimeRequest{
		Candidate: candidate, AuthorizationFingerprint: "policy:v1",
		Workspaces: []CandidateWorkspaceRuntime{
			{
				WorkspaceID: "dashboard", ServingStateID: "state_dashboard",
				ArtifactDigest: "sha256:" + strings.Repeat("b", 64), DataRevision: "snapshot:11",
				DataMode: CandidateDataReuseSnapshot,
			},
			{
				WorkspaceID: "sales", ServingStateID: "state_sales",
				ArtifactDigest: "sha256:" + strings.Repeat("c", 64), DataRevision: "snapshot:12",
				DataMode: CandidateDataRefreshSources,
				Connections: []CandidateConnectionRequirement{{
					LogicalConnectionID: "warehouse", ConnectorKind: "postgres",
				}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.RuntimeVersion != "leapview:test" || len(receipt.Workspaces) != 2 {
		t.Fatalf("runtime receipt = %#v", receipt)
	}
	if receipt.Workspaces[0].WorkspaceID != "dashboard" ||
		len(receipt.Workspaces[0].Bindings) != 0 ||
		receipt.Workspaces[1].WorkspaceID != "sales" ||
		len(receipt.Workspaces[1].Bindings) != 1 ||
		receipt.Workspaces[1].Bindings[0].BindingID != "binding_warehouse" {
		t.Fatalf("runtime receipt workspaces = %#v", receipt.Workspaces)
	}
	if len(connections.requests) != 2 || len(connections.requests[0].Requirements) != 0 ||
		len(connections.requests[1].Requirements) != 1 {
		t.Fatalf("connection requests = %#v", connections.requests)
	}
	for _, request := range connections.requests {
		if request.CandidateID != candidate.ID {
			t.Fatalf("connection candidate = %q, want %q", request.CandidateID, candidate.ID)
		}
	}
	if len(host.inputs) != 2 {
		t.Fatalf("runtime inputs = %#v", host.inputs)
	}
	if len(host.inputs[0].Registration.Compatibility.Bindings) != 0 {
		t.Fatalf("dashboard-only bindings = %#v", host.inputs[0].Registration.Compatibility.Bindings)
	}
	bindings := host.inputs[1].Registration.Compatibility.Bindings
	if len(bindings) != 1 || bindings[0].BindingID != "binding_warehouse" ||
		bindings[0].Revision != 7 || bindings[0].ProviderVersion != "provider:v3" {
		t.Fatalf("runtime binding evidence = %#v", bindings)
	}
	for _, input := range host.inputs {
		if input.Registration.CandidateID != candidate.ID ||
			input.Registration.OwnerID != candidate.OwnerID ||
			!input.Registration.ExpiresAt.Equal(candidate.ExpiresAt) ||
			input.Registration.Compatibility.AuthorizationFingerprint != "policy:v1" ||
			input.Registration.Compatibility.RuntimeVersion != "leapview:test" {
			t.Fatalf("candidate runtime input = %#v", input)
		}
	}
}

func TestCandidateRuntimeServiceReleasesPartialConnectionsOnFailure(t *testing.T) {
	now := time.Date(2026, 7, 29, 18, 0, 0, 0, time.UTC)
	connections := &candidateRuntimeConnections{failWorkspace: "sales"}
	host := &candidateRuntimeHost{}
	service, err := NewCandidateRuntimeService(CandidateRuntimeServiceConfig{
		Connections: connections, Runtime: host, RuntimeVersion: "leapview:test",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Prepare(t.Context(), CandidateRuntimeRequest{
		Candidate:                candidateRuntimeTestCandidate(t, now),
		AuthorizationFingerprint: "policy:v1",
		Workspaces: []CandidateWorkspaceRuntime{
			{
				WorkspaceID: "sales", ServingStateID: "state_sales",
				ArtifactDigest: "artifact-sales", DataRevision: "snapshot:11",
				DataMode: CandidateDataRefreshSources,
				Connections: []CandidateConnectionRequirement{{
					LogicalConnectionID: "warehouse", ConnectorKind: "postgres",
				}},
			},
			{
				WorkspaceID: "operations", ServingStateID: "state_operations",
				ArtifactDigest: "artifact-ops", DataRevision: "snapshot:12",
				DataMode: CandidateDataRefreshSources,
				Connections: []CandidateConnectionRequirement{{
					LogicalConnectionID: "warehouse", ConnectorKind: "postgres",
				}},
			},
		},
	})
	if !errors.Is(err, ErrCandidateUnavailable) {
		t.Fatalf("Prepare() error = %v", err)
	}
	if len(connections.leases) != 1 || connections.leases[0].closes != 1 {
		t.Fatalf("partial connection leases = %#v", connections.leases)
	}
	if len(host.inputs) != 0 {
		t.Fatalf("runtime host called after connection failure: %#v", host.inputs)
	}
}

func TestCandidateRuntimeServiceRejectsDuplicateNormalizedWorkspacesBeforeAcquiringConnections(t *testing.T) {
	now := time.Date(2026, 7, 29, 18, 0, 0, 0, time.UTC)
	connections := &candidateRuntimeConnections{}
	service, err := NewCandidateRuntimeService(CandidateRuntimeServiceConfig{
		Connections: connections, Runtime: &candidateRuntimeHost{},
		RuntimeVersion: "leapview:test",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Prepare(t.Context(), CandidateRuntimeRequest{
		Candidate:                candidateRuntimeTestCandidate(t, now),
		AuthorizationFingerprint: "policy:v1",
		Workspaces: []CandidateWorkspaceRuntime{
			{
				WorkspaceID: "sales", ServingStateID: "state_1",
				ArtifactDigest: "artifact-1", DataRevision: "snapshot:11",
				DataMode: CandidateDataReuseSnapshot,
			},
			{
				WorkspaceID: " sales ", ServingStateID: "state_2",
				ArtifactDigest: "artifact-2", DataRevision: "snapshot:12",
				DataMode: CandidateDataReuseSnapshot,
			},
		},
	})
	if !errors.Is(err, ErrCandidateInvalid) {
		t.Fatalf("Prepare() error = %v, want invalid candidate", err)
	}
	if len(connections.requests) != 0 {
		t.Fatalf("connection requests = %#v, want none", connections.requests)
	}
}

func TestCandidateRuntimeServiceRejectsDataModeAndConnectionMismatchBeforeAcquisition(t *testing.T) {
	now := time.Date(2026, 7, 29, 18, 0, 0, 0, time.UTC)
	for name, workspace := range map[string]CandidateWorkspaceRuntime{
		"reuse_with_connection": {
			WorkspaceID: "sales", ServingStateID: "state_1",
			ArtifactDigest: "artifact-1", DataRevision: "snapshot:11",
			DataMode: CandidateDataReuseSnapshot,
			Connections: []CandidateConnectionRequirement{{
				LogicalConnectionID: "warehouse", ConnectorKind: "postgres",
			}},
		},
		"refresh_without_connection": {
			WorkspaceID: "sales", ServingStateID: "state_1",
			ArtifactDigest: "artifact-1", DataRevision: "snapshot:11",
			DataMode: CandidateDataRefreshSources,
		},
	} {
		t.Run(name, func(t *testing.T) {
			connections := &candidateRuntimeConnections{}
			service, err := NewCandidateRuntimeService(CandidateRuntimeServiceConfig{
				Connections: connections, Runtime: &candidateRuntimeHost{},
				RuntimeVersion: "leapview:test",
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.Prepare(t.Context(), CandidateRuntimeRequest{
				Candidate:                candidateRuntimeTestCandidate(t, now),
				AuthorizationFingerprint: "policy:v1",
				Workspaces:               []CandidateWorkspaceRuntime{workspace},
			})
			if !errors.Is(err, ErrCandidateInvalid) {
				t.Fatalf("Prepare() error = %v, want invalid candidate", err)
			}
			if len(connections.requests) != 0 {
				t.Fatalf("connection requests = %#v, want none", connections.requests)
			}
		})
	}
}

func candidateRuntimeTestCandidate(t *testing.T, now time.Time) Candidate {
	t.Helper()
	candidate, err := NewCandidate(CandidateStartInput{
		ID: "cand_1", ProjectID: "project_1", TargetID: "target_1",
		Environment: "prod", OwnerID: "author_1",
		BaseGeneration: "deployment_1",
		ArtifactDigest: "sha256:" + strings.Repeat("a", 64),
		ExpiresAt:      now.Add(time.Hour), Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return candidate
}

type candidateRuntimeConnections struct {
	requests      []CandidateConnectionRequest
	leases        []*candidateRuntimeConnectionLeases
	failWorkspace string
}

func (connections *candidateRuntimeConnections) Acquire(
	_ context.Context,
	request CandidateConnectionRequest,
) (CandidateConnectionLeases, error) {
	connections.requests = append(connections.requests, request)
	if request.WorkspaceID == connections.failWorkspace {
		return nil, ErrCandidateUnavailable
	}
	evidence := []CandidateConnectionEvidence{}
	if len(request.Requirements) > 0 {
		evidence = append(evidence, CandidateConnectionEvidence{
			BindingID: "binding_warehouse", Revision: 7, ProviderVersion: "provider:v3",
		})
	}
	leases := &candidateRuntimeConnectionLeases{evidence: evidence}
	connections.leases = append(connections.leases, leases)
	return leases, nil
}

type candidateRuntimeConnectionLeases struct {
	evidence []CandidateConnectionEvidence
	closes   int
}

func (leases *candidateRuntimeConnectionLeases) Evidence() []CandidateConnectionEvidence {
	return append([]CandidateConnectionEvidence(nil), leases.evidence...)
}
func (leases *candidateRuntimeConnectionLeases) Close() error {
	leases.closes++
	return nil
}

type candidateRuntimeHost struct {
	inputs []runtimehost.CandidatePreparation
}

func (host *candidateRuntimeHost) PrepareAndRegisterCandidateSet(
	_ context.Context,
	inputs []runtimehost.CandidatePreparation,
) error {
	host.inputs = append([]runtimehost.CandidatePreparation(nil), inputs...)
	return nil
}
