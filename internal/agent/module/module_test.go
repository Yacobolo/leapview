package module

import (
	"path/filepath"
	"testing"

	"github.com/Yacobolo/leapview/internal/platform"
)

func TestBuildConstructsAgentServiceAndPersistence(t *testing.T) {
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "leapview.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	module, err := Build(t.Context(), Config{Database: store.SQLDB()})
	if err != nil {
		t.Fatal(err)
	}
	if module.service == nil || module.HTTP() == nil {
		t.Fatal("agent module did not construct its owned service and transport")
	}
}
