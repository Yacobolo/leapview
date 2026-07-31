package securefs

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
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

func TestWritePrivateFileAtomicConcurrentWritersLeaveOneCompletePrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	first := bytes.Repeat([]byte("first\n"), 1024)
	second := bytes.Repeat([]byte("second\n"), 1024)
	start := make(chan struct{})
	errors := make(chan error, 2)

	var writers sync.WaitGroup
	for _, contents := range [][]byte{first, second} {
		contents := contents
		writers.Add(1)
		go func() {
			defer writers.Done()
			<-start
			errors <- WritePrivateFileAtomic(path, contents)
		}()
	}
	close(start)
	writers.Wait()
	close(errors)

	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := ReadPrivateFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, first) && !bytes.Equal(got, second) {
		t.Fatalf("concurrent write left partial contents of %d bytes", len(got))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != PrivateFileMode {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain after concurrent writes: %v", matches)
	}
}
