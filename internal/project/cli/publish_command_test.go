package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apigenclient "github.com/Yacobolo/toolbelt/apigen/runtime/client"
	"github.com/flidai/leapview/internal/platform/cliapi"
)

func TestCandidateCheckpointStoreRoundTripsExactNonSecretIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authoring.json")
	store := NewCandidateCheckpointStore(path)
	projectPath := filepath.Join(t.TempDir(), "leapview.yaml")
	checkpoint := candidateCheckpoint(projectPath)

	if err := store.Save(checkpoint); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
	loaded, err := store.Load(projectPath, checkpoint.TargetOrigin)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != checkpoint {
		t.Fatalf("loaded = %#v, want %#v", loaded, checkpoint)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"secret-token", `"token"`, `"password"`} {
		if strings.Contains(strings.ToLower(string(content)), forbidden) {
			t.Fatalf("checkpoint persisted forbidden secret material: %s", content)
		}
	}
}

func TestCandidateCheckpointStoreFailsClosedForUnknownOrSecretFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authoring.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"candidates":{},"token":"secret-token"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewCandidateCheckpointStore(path)
	if _, err := store.Load("leapview.yaml", "https://target.example"); err == nil {
		t.Fatal("Load() accepted a secret-bearing unknown field")
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"candidates":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load("leapview.yaml", "https://target.example"); !errors.Is(err, ErrCandidateCheckpointNotFound) {
		t.Fatalf("Load() error = %v, want ErrCandidateCheckpointNotFound", err)
	}
	if err := os.WriteFile(
		path,
		[]byte(`{"version":1,"candidates":{}} {"version":1,"candidates":{}}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(
		"leapview.yaml",
		"https://target.example",
	); err == nil {
		t.Fatal("Load() accepted trailing JSON content")
	}
}

func TestPublishCommandUsesExactCheckpointWithoutReadingProjectSource(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "missing.yaml")
	store := NewCandidateCheckpointStore(filepath.Join(t.TempDir(), "authoring.json"))
	checkpoint := candidateCheckpoint(projectPath)
	if err := store.Save(checkpoint); err != nil {
		t.Fatal(err)
	}
	operations := &publishOperations{}
	command := PublishCommand(t.Context(), publishClient{}, store, operations)
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs([]string{"--project", projectPath, "--target", "enterprise"})

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if operations.options.Checkpoint != checkpoint {
		t.Fatalf("checkpoint = %#v, want %#v", operations.options.Checkpoint, checkpoint)
	}
	if operations.options.Credentials.Target != checkpoint.TargetOrigin ||
		operations.options.Credentials.Token != "ephemeral-token" {
		t.Fatalf("credentials = %#v", operations.options.Credentials)
	}
}

func TestPublishCommandRequiresPriorDevCandidateForResolvedTarget(t *testing.T) {
	store := NewCandidateCheckpointStore(filepath.Join(t.TempDir(), "authoring.json"))
	command := PublishCommand(t.Context(), publishClient{}, store, &publishOperations{})
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	command.SetArgs([]string{"--project", "missing.yaml", "--target", "enterprise"})

	err := command.Execute()
	if !errors.Is(err, ErrCandidateCheckpointNotFound) ||
		!strings.Contains(err.Error(), "leapview dev") {
		t.Fatalf("Execute() error = %v", err)
	}
}

type publishClient struct{}

func (publishClient) Resolve(context.Context, cliapi.Credentials) (cliapi.Credentials, error) {
	return cliapi.Credentials{Target: "https://target.example", Token: "ephemeral-token"}, nil
}

func (publishClient) Environment(context.Context, cliapi.Credentials, string) (string, error) {
	return "", nil
}

func (publishClient) Transport(context.Context, cliapi.Credentials) (apigenclient.Transport, error) {
	return nil, nil
}

type publishOperations struct {
	options PublishOptions
}

func (operations *publishOperations) Publish(
	_ context.Context,
	options PublishOptions,
	_ io.Writer,
) error {
	operations.options = options
	return nil
}

func candidateCheckpoint(projectPath string) CandidateCheckpoint {
	absolute, err := filepath.Abs(projectPath)
	if err != nil {
		panic(err)
	}
	return CandidateCheckpoint{
		ProjectPath: absolute, TargetOrigin: "https://target.example",
		TargetID: "target_1", Environment: "production", ProjectID: "finance",
		CandidateID: "cand_1", CandidateRevision: 7,
		ArtifactDigest:   "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ProvenanceDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
}
