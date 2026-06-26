package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Yacobolo/libredash/internal/config"
)

func TestLocalDevServerAlwaysOpensPlatformStore(t *testing.T) {
	home := t.TempDir()
	_, cleanup, err := localDevServer(context.Background(), nil, config.Config{HomeDir: home}, "test")
	if err != nil {
		t.Fatalf("local dev server: %v", err)
	}
	defer cleanup()

	if _, err := os.Stat(filepath.Join(home, "libredash.db")); err != nil {
		t.Fatalf("platform store was not created: %v", err)
	}
}
