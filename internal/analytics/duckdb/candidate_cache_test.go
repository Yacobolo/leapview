package duckdb

import "testing"

func TestWorkspaceQueryCacheNamespacePartitionsCandidateSecurityBoundaries(t *testing.T) {
	base := WorkspaceRuntimeConfig{
		SnapshotID: 11, ServingStateID: "state_1", WorkspaceID: "sales",
		Environment: "prod", SemanticDigest: "semantic", ArtifactDigest: "artifact",
		SourceDataDigest: "data", CandidateID: "cand_1",
		AuthorizationFingerprint: "policy-a", BindingFingerprint: "bindings-a",
	}
	namespace := workspaceQueryCacheNamespace(base)
	for name, mutate := range map[string]func(*WorkspaceRuntimeConfig){
		"candidate": func(config *WorkspaceRuntimeConfig) {
			config.CandidateID = "cand_2"
		},
		"authorization": func(config *WorkspaceRuntimeConfig) {
			config.AuthorizationFingerprint = "policy-b"
		},
		"binding": func(config *WorkspaceRuntimeConfig) {
			config.BindingFingerprint = "bindings-b"
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := base
			mutate(&changed)
			if got := workspaceQueryCacheNamespace(changed); got == namespace {
				t.Fatalf("%s boundary reused candidate cache namespace %q", name, got)
			}
		})
	}
}
