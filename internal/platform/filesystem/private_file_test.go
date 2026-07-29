package securefs

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWritePrivateFileAtomicRoundTripsWithPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "credentials.json")
	contents := []byte("secret\n")
	if err := WritePrivateFileAtomic(path, contents); err != nil {
		t.Fatal(err)
	}
	got, err := ReadPrivateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, contents) {
		t.Fatalf("contents = %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != PrivateFileMode {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}

func TestReadPrivateFileRejectsBroadPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPrivateFile(path); err == nil {
		t.Fatal("broadly readable private file was accepted")
	}
}
