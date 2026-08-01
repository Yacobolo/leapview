package credentialfiles_test

import (
	"os"
	"testing"

	"github.com/flidai/leapview/rtest/internal/credentialfiles"
)

func TestWriteUsesPrivateTemporaryFilesAndCleanup(t *testing.T) {
	files, err := credentialfiles.Write([]byte("ca"), []byte("cert"), []byte("key"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{files.CA, files.Certificate, files.Key} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %o", path, info.Mode().Perm())
		}
	}
	root := files.Root
	if err := files.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("temporary credential directory remains: %v", err)
	}
}
