package filesystem_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yacobolo/libredash/internal/manageddata/storage"
	"github.com/Yacobolo/libredash/internal/manageddata/storage/filesystem"
	"github.com/Yacobolo/libredash/internal/manageddata/storage/storagetest"
)

func TestBlobStoreConformance(t *testing.T) {
	storagetest.BlobStoreConformance(t, func(t *testing.T) storage.BlobStore {
		store, err := filesystem.New(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		return store
	})
}

func TestStoreUsesPrivatePermissionsAndContentAddressedPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "managed")
	store, err := filesystem.New(root)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("private")
	blob := testBlob(body)
	if _, err := store.Put(t.Context(), blob, bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}

	path := store.BlobPath(blob.SHA256)
	if filepath.Dir(path) != filepath.Join(root, "blobs", "sha256", blob.SHA256[:2]) {
		t.Fatalf("BlobPath() = %q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o400 {
		t.Fatalf("blob permissions = %o, want 400", got)
	}
	rootInfo, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := rootInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("root permissions = %o, want 700", got)
	}
}

func TestMaterializeRevisionCreatesImmutableHardLinkedView(t *testing.T) {
	root := t.TempDir()
	t.Cleanup(func() {
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err == nil && entry.IsDir() {
				_ = os.Chmod(path, 0o700)
			}
			return nil
		})
	})
	store, err := filesystem.New(root)
	if err != nil {
		t.Fatal(err)
	}
	one := testBlob([]byte("one"))
	two := testBlob([]byte("two"))
	for _, item := range []struct {
		blob storage.Blob
		body []byte
	}{{one, []byte("one")}, {two, []byte("two")}} {
		if _, err := store.Put(t.Context(), item.blob, bytes.NewReader(item.body)); err != nil {
			t.Fatal(err)
		}
	}

	files := []storage.RevisionFile{
		{Path: "orders/one.csv", SHA256: one.SHA256},
		{Path: "two.csv", SHA256: two.SHA256},
	}
	first, err := store.MaterializeRevision(t.Context(), "sha256:"+one.SHA256, files)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.MaterializeRevision(t.Context(), "sha256:"+one.SHA256, files)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("idempotent MaterializeRevision() = %#v, %#v", first, second)
	}

	viewInfo, err := os.Stat(first.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got := viewInfo.Mode().Perm(); got != 0o500 {
		t.Fatalf("revision permissions = %o, want 500", got)
	}
	sourceInfo, _ := os.Stat(store.BlobPath(one.SHA256))
	viewFileInfo, _ := os.Stat(filepath.Join(first.Path, "orders", "one.csv"))
	if !os.SameFile(sourceInfo, viewFileInfo) {
		t.Fatal("revision file is not a hard link to its blob")
	}
	if err := os.WriteFile(filepath.Join(first.Path, "orders", "one.csv"), []byte("mutation"), 0o600); err == nil {
		t.Fatal("immutable revision file was writable")
	}

	if err := store.DeleteUnreachable(t.Context(), []string{one.SHA256}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(first.Path, "orders", "one.csv"))
	if err != nil || string(got) != "one" {
		t.Fatalf("hard-linked view after blob deletion = %q, %v", got, err)
	}
	third, err := store.MaterializeRevision(t.Context(), "sha256:"+one.SHA256, files)
	if err != nil || third != first {
		t.Fatalf("MaterializeRevision() after blob deletion = %#v, %v", third, err)
	}
}

func TestMaterializeRevisionRejectsUnsafePathsAndMissingBlobs(t *testing.T) {
	store, err := filesystem.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	digest := testBlob([]byte("missing")).SHA256
	tests := []struct {
		name       string
		revisionID string
		path       string
		want       error
	}{
		{name: "revision traversal", revisionID: "../revision", path: "file.csv", want: storage.ErrInvalid},
		{name: "logical traversal", revisionID: "sha256:" + digest, path: "../file.csv", want: storage.ErrInvalid},
		{name: "missing blob", revisionID: "sha256:" + digest, path: "file.csv", want: storage.ErrNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := store.MaterializeRevision(t.Context(), test.revisionID, []storage.RevisionFile{{Path: test.path, SHA256: digest}})
			if !errors.Is(err, test.want) {
				t.Fatalf("MaterializeRevision() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestMaterializeRevisionRequiresCanonicalManagedRevisionID(t *testing.T) {
	store, err := filesystem.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	digest := testBlob([]byte("revision")).SHA256
	for _, revisionID := range []string{
		digest,
		"sha256:" + strings.ToUpper(digest),
		"sha256:short",
		"md5:" + digest,
	} {
		t.Run(revisionID, func(t *testing.T) {
			_, err := store.MaterializeRevision(t.Context(), revisionID, nil)
			if !errors.Is(err, storage.ErrInvalid) {
				t.Fatalf("MaterializeRevision(%q) error = %v", revisionID, err)
			}
		})
	}
}

func testBlob(body []byte) storage.Blob {
	sum := sha256.Sum256(body)
	return storage.Blob{SHA256: hex.EncodeToString(sum[:]), Size: int64(len(body))}
}
