package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Yacobolo/leapview/internal/app/config"
)

func TestApplicationAssetsResolveConfiguredAndGeneratedVersions(t *testing.T) {
	configured := applicationAssets(config.Config{AssetVersion: " release-123 "}, true)
	if got := configured.Version(); got != "release-123" {
		t.Fatalf("configured Version = %q, want release-123", got)
	}
	if !configured.Production() {
		t.Fatal("configured resolver is not production")
	}

	root := t.TempDir()
	versionPath := filepath.Join(root, "static", "asset-version.txt")
	if err := os.MkdirAll(filepath.Dir(versionPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(versionPath, []byte("generated-456\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	generated := applicationAssets(config.Config{}, true)
	if got := generated.Version(); got != "generated-456" {
		t.Fatalf("generated Version = %q, want generated-456", got)
	}
}
