package recovery

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type fakeSnapshot struct{ data []byte }

func (f fakeSnapshot) Backup(_ context.Context, output string) error {
	return os.WriteFile(output, f.data, 0o600)
}

func TestCreateAndRestoreValidatedBundle(t *testing.T) {
	source := filepath.Join(t.TempDir(), "state")
	writeFile(t, filepath.Join(source, "control", "token-pepper"), "pepper")
	writeFile(t, filepath.Join(source, "pki", "ca.pem"), "authority")
	writeFile(t, filepath.Join(source, "pki", "ca-key.pem"), "private")
	bundle := filepath.Join(t.TempDir(), "backup")
	if err := Create(context.Background(), fakeSnapshot{data: []byte("database")}, source, bundle); err != nil {
		t.Fatal(err)
	}

	restored := filepath.Join(t.TempDir(), "restored")
	if err := Restore(bundle, restored); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"control/control.db": "database", "control/token-pepper": "pepper", "pki/ca.pem": "authority", "pki/ca-key.pem": "private",
	} {
		data, err := os.ReadFile(filepath.Join(restored, filepath.FromSlash(name)))
		if err != nil || string(data) != want {
			t.Fatalf("%s = %q, %v", name, data, err)
		}
	}
}

func TestRestoreRejectsTamperedBundle(t *testing.T) {
	source := filepath.Join(t.TempDir(), "state")
	writeFile(t, filepath.Join(source, "control", "token-pepper"), "pepper")
	writeFile(t, filepath.Join(source, "pki", "ca.pem"), "authority")
	writeFile(t, filepath.Join(source, "pki", "ca-key.pem"), "private")
	bundle := filepath.Join(t.TempDir(), "backup")
	if err := Create(context.Background(), fakeSnapshot{data: []byte("database")}, source, bundle); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(bundle, "control", "control.db"), "tampered")
	if err := Restore(bundle, filepath.Join(t.TempDir(), "restored")); err == nil {
		t.Fatal("tampered bundle was restored")
	}
}

func writeFile(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}
