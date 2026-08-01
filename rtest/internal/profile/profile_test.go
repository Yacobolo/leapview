package profile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flidai/leapview/rtest/internal/profile"
)

func TestLoadResolvesNamedSuiteFromRepositoryRoot(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".rtest.json"), `{
  "repository": "example/service",
  "runner": "standard",
  "suites": {
    "integration": {
      "command": ["go", "test", "-count=1", "-tags=integration", "./..."],
      "timeout_seconds": 900
    }
  }
}`)
	nested := filepath.Join(root, "internal", "feature")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	resolved, err := profile.Load(root, "integration", []string{"./internal/feature"})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Repository != "example/service" || resolved.Runner != "standard" || resolved.TimeoutSeconds != 900 {
		t.Fatalf("resolved = %#v", resolved)
	}
	wantLast := "./internal/feature"
	if got := resolved.Command[len(resolved.Command)-1]; got != wantLast {
		t.Fatalf("last command argument = %q, want %q", got, wantLast)
	}
}

func TestLoadRejectsUnknownSuite(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, ".rtest.json"), `{
  "repository": "example/service",
  "suites": {"test": {"command": ["go", "test", "./..."]}}
}`)
	if _, err := profile.Load(root, "missing", nil); err == nil {
		t.Fatal("Load succeeded for an unknown suite")
	}
}

func write(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}
