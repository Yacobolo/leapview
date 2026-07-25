package staticasset

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVersionDefaultsToDevOutsideProduction(t *testing.T) {
	assets := New(Config{})
	if got := assets.Version(); got != "dev" {
		t.Fatalf("Version = %q, want dev", got)
	}
}

func TestVersionUsesConfiguredOverride(t *testing.T) {
	assets := New(Config{Production: true, Version: " release-123 "})
	if got := assets.URL("/static/app-shell.js"); got != "/static/app-shell.js?v=release-123" {
		t.Fatalf("URL = %q, want configured version", got)
	}
}

func TestVersionUsesGeneratedFileInProduction(t *testing.T) {
	dir := t.TempDir()
	versionPath := filepath.Join(dir, "static", "asset-version.txt")
	if err := os.MkdirAll(filepath.Dir(versionPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(versionPath, []byte("abc123\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	assets := New(Config{Production: true, GeneratedVersionPath: versionPath})
	if got := assets.URL("/static/app-shell.js"); got != "/static/app-shell.js?v=abc123" {
		t.Fatalf("URL = %q, want generated version", got)
	}
}

func TestProductionUsesInjectedValue(t *testing.T) {
	if !New(Config{Production: true}).Production() {
		t.Fatal("Production() = false, want true")
	}
	if New(Config{}).Production() {
		t.Fatal("Production() = true for invalid boolean, want false")
	}
}

func TestResolverIsImmutableAfterConstruction(t *testing.T) {
	versionPath := filepath.Join(t.TempDir(), "asset-version.txt")
	if err := os.WriteFile(versionPath, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	assets := New(Config{Production: true, GeneratedVersionPath: versionPath})
	if err := os.WriteFile(versionPath, []byte("second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := assets.Version(); got != "first" {
		t.Fatalf("Version = %q, want immutable first", got)
	}
}
