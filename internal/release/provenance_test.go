package release

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestProvenanceSeparatesImmutableArtifactFromTargetPlan(t *testing.T) {
	first, err := NewProvenance(provenanceInput("target-dev", "dev", "a"))
	if err != nil {
		t.Fatal(err)
	}
	reordered := provenanceInput("target-dev", "dev", "a")
	reordered.Artifact.Workspaces[0], reordered.Artifact.Workspaces[1] =
		reordered.Artifact.Workspaces[1], reordered.Artifact.Workspaces[0]
	reordered.Plan.Workspaces[0], reordered.Plan.Workspaces[1] =
		reordered.Plan.Workspaces[1], reordered.Plan.Workspaces[0]
	second, err := NewProvenance(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != second.Digest ||
		first.ArtifactDigest != second.ArtifactDigest ||
		first.PlanDigest != second.PlanDigest {
		t.Fatalf("canonical provenance changed with input order: %#v / %#v", first, second)
	}

	promoted, err := NewProvenance(provenanceInput("target-prod", "prod", "b"))
	if err != nil {
		t.Fatal(err)
	}
	if promoted.ArtifactDigest != first.ArtifactDigest {
		t.Fatalf("promotion changed artifact digest: %q / %q", first.ArtifactDigest, promoted.ArtifactDigest)
	}
	if promoted.PlanDigest == first.PlanDigest || promoted.Digest == first.Digest {
		t.Fatalf("target-specific plan did not change release identity: %#v / %#v", first, promoted)
	}
	if err := first.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestProvenanceFailsClosedOnTamperingAndIncompleteCompatibility(t *testing.T) {
	valid, err := NewProvenance(provenanceInput("target-dev", "dev", "a"))
	if err != nil {
		t.Fatal(err)
	}
	tampered := valid
	tampered.Artifact.ProjectDigest = shaIdentity("f")
	if err := tampered.Validate(); !errors.Is(err, ErrProvenanceInvalid) {
		t.Fatalf("tampered provenance error = %v, want ErrProvenanceInvalid", err)
	}

	tests := map[string]func(*ProvenanceInput){
		"compiler version": func(input *ProvenanceInput) {
			input.Artifact.CompilerVersion = ""
		},
		"artifact schema": func(input *ProvenanceInput) {
			input.Artifact.SchemaVersion = 0
		},
		"managed data pin": func(input *ProvenanceInput) {
			input.Plan.Workspaces[0].ManagedDataPins[0].RevisionID = ""
		},
		"binding evidence": func(input *ProvenanceInput) {
			input.Plan.Workspaces[1].Bindings[0].ValidatedVersion = ""
		},
		"runtime version": func(input *ProvenanceInput) {
			input.Plan.RuntimeVersion = ""
		},
		"policy digest": func(input *ProvenanceInput) {
			input.Plan.PolicyDigest = ""
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			input := provenanceInput("target-dev", "dev", "a")
			mutate(&input)
			if _, err := NewProvenance(input); !errors.Is(err, ErrProvenanceInvalid) {
				t.Fatalf("NewProvenance() error = %v, want ErrProvenanceInvalid", err)
			}
		})
	}
}

func TestProvenanceSerializesOnlyRedactedTargetEvidence(t *testing.T) {
	provenance, err := NewProvenance(provenanceInput("target-dev", "dev", "a"))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(provenance)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"infisicalProject", "secretPath", "secretKey", "credential",
		"postgres://", "super-secret",
	} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("provenance contains forbidden provider or secret material %q: %s", forbidden, encoded)
		}
	}
}

func provenanceInput(targetID, environment, suffix string) ProvenanceInput {
	return ProvenanceInput{
		Artifact: ProjectArtifactProvenance{
			SourceDigest:    shaIdentity("1"),
			ProjectDigest:   shaIdentity("2"),
			CompilerVersion: "leapview:1.2.3",
			SchemaVersion:   3,
			Workspaces: []WorkspaceArtifactProvenance{
				{WorkspaceID: "sales", ArtifactDigest: shaIdentity("3")},
				{WorkspaceID: "operations", ArtifactDigest: shaIdentity("4")},
			},
		},
		Candidate: CandidateProvenance{
			ID: "cand_1", Revision: 7, OwnerID: "principal_1",
		},
		Plan: TargetPlanProvenance{
			TargetID: targetID, Environment: environment,
			BaseGeneration: "generation-" + suffix,
			RuntimeVersion: "leapview-runtime:1.2.3",
			PolicyDigest:   shaIdentity("5"),
			Workspaces: []TargetWorkspacePlan{
				{
					WorkspaceID: "sales", ServingStateID: "state-sales-" + suffix,
					ArtifactDigest: shaIdentity(suffix), DataRevision: "snapshot:42",
					DataMode: TargetDataReuseSnapshot,
					ManagedDataPins: []ManagedDataPin{
						{ConnectionID: "warehouse", RevisionID: shaIdentity("6")},
					},
				},
				{
					WorkspaceID: "operations", ServingStateID: "state-operations-" + suffix,
					ArtifactDigest: shaIdentity(suffix), DataRevision: "sources:" + shaIdentity("1"),
					DataMode: TargetDataRefreshSources,
					ManagedDataPins: []ManagedDataPin{
						{ConnectionID: "warehouse", RevisionID: shaIdentity("7")},
					},
					Bindings: []BindingEvidence{
						{BindingID: "warehouse", Revision: 2, ValidatedVersion: "version-9"},
					},
				},
			},
		},
	}
}

func shaIdentity(character string) string {
	return "sha256:" + strings.Repeat(character, 64)
}
