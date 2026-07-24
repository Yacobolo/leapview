package protocol

import (
	"path/filepath"
	"testing"

	"github.com/Yacobolo/leapview/internal/platform"
)

func TestBuildConstructsProtocolPersistence(t *testing.T) {
	store, err := platform.Open(t.Context(), filepath.Join(t.TempDir(), "leapview.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	protocol, err := Build(t.Context(), Config{Database: store.SQLDB()})
	if err != nil {
		t.Fatal(err)
	}
	if protocol.store == nil {
		t.Fatal("API protocol did not construct its idempotency store")
	}
}
