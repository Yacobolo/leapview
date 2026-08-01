package projectlink_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flidai/leapview/rtest/internal/projectlink"
)

func TestResolveUsesNearestRepositoryLink(t *testing.T) {
	root := gitRoot(t)
	write(t, filepath.Join(root, projectlink.FileName), `{"project":"root-project"}`)
	nested := filepath.Join(root, "services", "api")
	deeper := filepath.Join(nested, "internal")
	if err := os.MkdirAll(deeper, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(nested, projectlink.FileName), `{"project":"api-project"}`)

	got, err := projectlink.Resolve(context.Background(), deeper, "", "")
	if err != nil || got != "api-project" {
		t.Fatalf("nested project=%q err=%v", got, err)
	}
	other := filepath.Join(root, "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err = projectlink.Resolve(context.Background(), other, "", "")
	if err != nil || got != "root-project" {
		t.Fatalf("ancestor project=%q err=%v", got, err)
	}
}

func TestResolvePrecedenceIsFlagThenEnvironmentThenRepository(t *testing.T) {
	root := gitRoot(t)
	write(t, filepath.Join(root, projectlink.FileName), `{"project":"repository-project"}`)
	for _, test := range []struct {
		name, explicit, environment, want string
	}{
		{"flag", "flag-project", "environment-project", "flag-project"},
		{"environment", "", "environment-project", "environment-project"},
		{"repository", "", "", "repository-project"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := projectlink.Resolve(context.Background(), root, test.explicit, test.environment)
			if err != nil || got != test.want {
				t.Fatalf("project=%q err=%v", got, err)
			}
		})
	}
}

func TestResolveRejectsMissingMalformedAndUnknownRepositoryLinks(t *testing.T) {
	root := gitRoot(t)
	if _, err := projectlink.Resolve(context.Background(), root, "", ""); err == nil || !strings.Contains(err.Error(), projectlink.FileName) {
		t.Fatalf("missing error = %v", err)
	}
	write(t, filepath.Join(root, projectlink.FileName), `{"project":"one","token":"secret"}`)
	if _, err := projectlink.Resolve(context.Background(), root, "", ""); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field error = %v", err)
	}
	write(t, filepath.Join(root, projectlink.FileName), `{"project":`)
	if _, err := projectlink.Resolve(context.Background(), root, "", ""); err == nil || !strings.Contains(err.Error(), "read") {
		t.Fatalf("malformed error = %v", err)
	}
	write(t, filepath.Join(root, projectlink.FileName), `{"project":"one","project":"two"}`)
	if _, err := projectlink.Resolve(context.Background(), root, "", ""); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate project error = %v", err)
	}
}

func TestWriteIsCommitSafeAndRefusesConflictingRelink(t *testing.T) {
	root := gitRoot(t)
	path, err := projectlink.Write(root, "example-project")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{\n  \"project\": \"example-project\"\n}\n" || strings.Contains(strings.ToLower(string(data)), "token") {
		t.Fatalf("repository link = %q", data)
	}
	if _, err := projectlink.Write(root, "example-project"); err != nil {
		t.Fatalf("idempotent write: %v", err)
	}
	if _, err := projectlink.Write(root, "different-project"); err == nil || !strings.Contains(err.Error(), "already links") {
		t.Fatalf("conflicting write error = %v", err)
	}
}

func gitRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	command := exec.Command("git", "init", "-q")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	return root
}

func write(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}
