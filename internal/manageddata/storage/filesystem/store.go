// Package filesystem implements private, content-addressed managed-data storage.
package filesystem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Yacobolo/libredash/internal/manageddata/storage"
)

const (
	privateDir   = 0o700
	readOnlyDir  = 0o500
	privateFile  = 0o600
	readOnlyFile = 0o400
)

type Store struct {
	root      string
	blobs     string
	staging   string
	revisions string
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("%w: filesystem root is required", storage.ErrInvalid)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve filesystem root", storage.ErrInvalid)
	}
	store := &Store{
		root:      absRoot,
		blobs:     filepath.Join(absRoot, "blobs", "sha256"),
		staging:   filepath.Join(absRoot, "staging"),
		revisions: filepath.Join(absRoot, "revisions"),
	}
	for _, directory := range []string{store.root, filepath.Join(store.root, "blobs"), store.blobs, store.staging, store.revisions} {
		if err := mkdirPrivate(directory); err != nil {
			return nil, fmt.Errorf("initialize filesystem blob store: %w", err)
		}
	}
	return store, nil
}

func (s *Store) Put(ctx context.Context, expected storage.Blob, content io.Reader) (storage.Blob, error) {
	if err := storage.ValidateBlob(expected); err != nil {
		return storage.Blob{}, err
	}
	if content == nil {
		return storage.Blob{}, fmt.Errorf("%w: blob content is required", storage.ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return storage.Blob{}, err
	}
	target := s.blobPath(expected.SHA256)
	if _, err := os.Stat(target); err == nil {
		return s.verifyPath(ctx, target, expected)
	} else if !errors.Is(err, os.ErrNotExist) {
		return storage.Blob{}, fmt.Errorf("stat filesystem blob: %w", err)
	}

	temporary, err := os.CreateTemp(s.staging, "blob-*")
	if err != nil {
		return storage.Blob{}, fmt.Errorf("create filesystem blob staging file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(privateFile); err != nil {
		temporary.Close()
		return storage.Blob{}, fmt.Errorf("set filesystem blob staging permissions: %w", err)
	}

	hash := sha256.New()
	written, copyErr := copyExpected(ctx, io.MultiWriter(temporary, hash), content, expected.Size)
	if copyErr != nil {
		temporary.Close()
		return storage.Blob{}, copyErr
	}
	actualDigest := hex.EncodeToString(hash.Sum(nil))
	if written != expected.Size || actualDigest != expected.SHA256 {
		temporary.Close()
		return storage.Blob{}, fmt.Errorf("%w: expected size %d and SHA-256 %s, received size %d and SHA-256 %s", storage.ErrIntegrity, expected.Size, expected.SHA256, written, actualDigest)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return storage.Blob{}, fmt.Errorf("sync filesystem blob staging file: %w", err)
	}
	if err := temporary.Chmod(readOnlyFile); err != nil {
		temporary.Close()
		return storage.Blob{}, fmt.Errorf("set filesystem blob permissions: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return storage.Blob{}, fmt.Errorf("close filesystem blob staging file: %w", err)
	}

	if err := mkdirPrivate(filepath.Dir(target)); err != nil {
		return storage.Blob{}, fmt.Errorf("create filesystem blob directory: %w", err)
	}
	if err := os.Link(temporaryPath, target); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return storage.Blob{}, fmt.Errorf("atomically finalize filesystem blob: %w", err)
		}
		return s.verifyPath(ctx, target, expected)
	}
	if err := syncDirectory(filepath.Dir(target)); err != nil {
		return storage.Blob{}, fmt.Errorf("sync filesystem blob directory: %w", err)
	}
	return s.verifyPath(ctx, target, expected)
}

func (s *Store) Stat(ctx context.Context, digest string) (storage.Blob, error) {
	if err := storage.ValidateSHA256(digest); err != nil {
		return storage.Blob{}, err
	}
	return s.verifyPath(ctx, s.blobPath(digest), storage.Blob{SHA256: digest, Size: -1})
}

func (s *Store) Open(ctx context.Context, digest string) (io.ReadCloser, error) {
	if _, err := s.Stat(ctx, digest); err != nil {
		return nil, err
	}
	file, err := os.Open(s.blobPath(digest))
	if errors.Is(err, os.ErrNotExist) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("open filesystem blob: %w", err)
	}
	return file, nil
}

func (s *Store) WalkBlobs(ctx context.Context, visit func(storage.BlobMetadata) error) error {
	if visit == nil {
		return fmt.Errorf("%w: blob visitor is required", storage.ErrInvalid)
	}
	shards, err := os.ReadDir(s.blobs)
	if err != nil {
		return filesystemInventoryError(ctx, "enumerate blob shards", err)
	}
	for _, shard := range shards {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !isCanonicalShard(shard.Name()) || shard.Type()&os.ModeSymlink != 0 || !shard.IsDir() {
			return fmt.Errorf("%w: filesystem blob inventory is noncanonical", storage.ErrIntegrity)
		}
		files, err := os.ReadDir(filepath.Join(s.blobs, shard.Name()))
		if err != nil {
			return filesystemInventoryError(ctx, "enumerate blob shard", err)
		}
		for _, entry := range files {
			if err := ctx.Err(); err != nil {
				return err
			}
			digest := entry.Name()
			if storage.ValidateSHA256(digest) != nil || digest[:2] != shard.Name() || entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("%w: filesystem blob inventory is noncanonical", storage.ErrIntegrity)
			}
			info, err := entry.Info()
			if err != nil {
				return filesystemInventoryError(ctx, "inspect blob metadata", err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("%w: filesystem blob inventory contains a non-file", storage.ErrIntegrity)
			}
			if err := visit(storage.BlobMetadata{SHA256: digest, Size: info.Size(), LastModified: info.ModTime().UTC()}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) DeleteBlobs(ctx context.Context, digests []string) error {
	if len(digests) > 1000 {
		return fmt.Errorf("%w: blob deletion batch exceeds 1000 entries", storage.ErrInvalid)
	}
	for _, digest := range digests {
		if err := storage.ValidateSHA256(digest); err != nil {
			return err
		}
	}
	touched := make(map[string]struct{})
	for _, digest := range digests {
		if err := ctx.Err(); err != nil {
			return err
		}
		blobPath := s.blobPath(digest)
		info, err := os.Lstat(blobPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return filesystemInventoryError(ctx, "inspect blob for deletion", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("%w: filesystem blob deletion target is noncanonical", storage.ErrIntegrity)
		}
		if err := os.Remove(blobPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return filesystemInventoryError(ctx, "delete blob", err)
		}
		touched[filepath.Dir(blobPath)] = struct{}{}
	}
	for directory := range touched {
		if err := syncDirectory(directory); err != nil {
			return filesystemInventoryError(ctx, "sync blob deletion", err)
		}
	}
	return nil
}

func (s *Store) MaterializeRevision(ctx context.Context, revisionID string, files []storage.RevisionFile) (storage.RevisionView, error) {
	if err := validateRevisionID(revisionID); err != nil {
		return storage.RevisionView{}, err
	}
	if err := validateRevisionFiles(files); err != nil {
		return storage.RevisionView{}, err
	}
	finalPath := filepath.Join(s.revisions, revisionID)
	if _, err := os.Stat(finalPath); err == nil {
		if err := s.verifyRevision(ctx, finalPath, files); err != nil {
			return storage.RevisionView{}, err
		}
		return storage.RevisionView{ID: revisionID, Path: finalPath}, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return storage.RevisionView{}, fmt.Errorf("stat filesystem revision: %w", err)
	}

	temporary, err := os.MkdirTemp(s.revisions, ".revision-*")
	if err != nil {
		return storage.RevisionView{}, fmt.Errorf("create filesystem revision staging directory: %w", err)
	}
	cleanup := func() {
		_ = makeTreeWritable(temporary)
		_ = os.RemoveAll(temporary)
	}
	defer cleanup()
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return storage.RevisionView{}, err
		}
		blob, err := s.Stat(ctx, file.SHA256)
		if err != nil {
			return storage.RevisionView{}, err
		}
		destination := filepath.Join(temporary, filepath.FromSlash(file.Path))
		if err := mkdirPrivate(filepath.Dir(destination)); err != nil {
			return storage.RevisionView{}, fmt.Errorf("create filesystem revision directory: %w", err)
		}
		if err := os.Link(s.blobPath(blob.SHA256), destination); err != nil {
			return storage.RevisionView{}, fmt.Errorf("link filesystem revision blob: %w", err)
		}
	}
	if err := os.Rename(temporary, finalPath); err != nil {
		if _, statErr := os.Stat(finalPath); statErr == nil {
			if verifyErr := s.verifyRevision(ctx, finalPath, files); verifyErr != nil {
				return storage.RevisionView{}, verifyErr
			}
			return storage.RevisionView{ID: revisionID, Path: finalPath}, nil
		}
		return storage.RevisionView{}, fmt.Errorf("atomically finalize filesystem revision: %w", err)
	}
	temporary = ""
	if err := makeTreeReadOnly(finalPath); err != nil {
		return storage.RevisionView{}, fmt.Errorf("make filesystem revision immutable: %w", err)
	}
	if err := syncDirectory(s.revisions); err != nil {
		return storage.RevisionView{}, fmt.Errorf("sync filesystem revision directory: %w", err)
	}
	return storage.RevisionView{ID: revisionID, Path: finalPath}, nil
}

func (s *Store) BlobPath(digest string) string {
	if storage.ValidateSHA256(digest) != nil {
		return ""
	}
	return s.blobPath(digest)
}

func (s *Store) blobPath(digest string) string {
	return filepath.Join(s.blobs, digest[:2], digest)
}

func isCanonicalShard(value string) bool {
	if len(value) != 2 {
		return false
	}
	for _, char := range []byte(value) {
		if char < '0' || char > '9' && char < 'a' || char > 'f' {
			return false
		}
	}
	return true
}

func filesystemInventoryError(ctx context.Context, operation string, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return fmt.Errorf("%w: %s", storage.ErrBackend, operation)
}

func (s *Store) verifyPath(ctx context.Context, filePath string, expected storage.Blob) (storage.Blob, error) {
	file, err := os.Open(filePath)
	if errors.Is(err, os.ErrNotExist) {
		return storage.Blob{}, storage.ErrNotFound
	}
	if err != nil {
		return storage.Blob{}, fmt.Errorf("open filesystem blob for verification: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return storage.Blob{}, fmt.Errorf("stat filesystem blob: %w", err)
	}
	if !info.Mode().IsRegular() {
		return storage.Blob{}, fmt.Errorf("%w: filesystem blob is not a regular file", storage.ErrIntegrity)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, &contextReader{ctx: ctx, reader: file}); err != nil {
		return storage.Blob{}, fmt.Errorf("verify filesystem blob: %w", err)
	}
	actualDigest := hex.EncodeToString(hash.Sum(nil))
	if actualDigest != expected.SHA256 || expected.Size >= 0 && info.Size() != expected.Size {
		return storage.Blob{}, fmt.Errorf("%w: filesystem blob does not match its content address", storage.ErrIntegrity)
	}
	return storage.Blob{SHA256: actualDigest, Size: info.Size(), URI: (&url.URL{Scheme: "file", Path: filePath}).String()}, nil
}

func (s *Store) verifyRevision(ctx context.Context, revisionPath string, files []storage.RevisionFile) error {
	expected := make(map[string]string, len(files))
	for _, file := range files {
		expected[file.Path] = file.SHA256
	}
	seen := 0
	err := filepath.WalkDir(revisionPath, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(revisionPath, current)
		if err != nil {
			return err
		}
		logicalPath := filepath.ToSlash(relative)
		digest, ok := expected[logicalPath]
		if !ok {
			return fmt.Errorf("%w: filesystem revision contains unexpected path %q", storage.ErrIntegrity, logicalPath)
		}
		sourceInfo, err := os.Stat(s.blobPath(digest))
		sourceExists := err == nil
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		viewInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if !viewInfo.Mode().IsRegular() {
			return fmt.Errorf("%w: filesystem revision path %q is not a regular file", storage.ErrIntegrity, logicalPath)
		}
		if sourceExists && !os.SameFile(sourceInfo, viewInfo) {
			return fmt.Errorf("%w: filesystem revision path %q is not linked to its blob", storage.ErrIntegrity, logicalPath)
		}
		viewFile, err := os.Open(current)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, &contextReader{ctx: ctx, reader: viewFile})
		closeErr := viewFile.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if hex.EncodeToString(hash.Sum(nil)) != digest {
			return fmt.Errorf("%w: filesystem revision path %q does not match its content address", storage.ErrIntegrity, logicalPath)
		}
		seen++
		return nil
	})
	if err != nil {
		return err
	}
	if seen != len(expected) {
		return fmt.Errorf("%w: filesystem revision is missing files", storage.ErrIntegrity)
	}
	return nil
}

func copyExpected(ctx context.Context, destination io.Writer, source io.Reader, expectedSize int64) (int64, error) {
	reader := &contextReader{ctx: ctx, reader: source}
	if expectedSize < int64(^uint64(0)>>1) {
		reader.reader = io.LimitReader(reader.reader, expectedSize+1)
	}
	written, err := io.Copy(destination, reader)
	if err != nil {
		return written, fmt.Errorf("write filesystem blob: %w", err)
	}
	return written, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

func validateRevisionID(value string) error {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) {
		return fmt.Errorf("%w: revision ID must use the sha256 scheme", storage.ErrInvalid)
	}
	if err := storage.ValidateSHA256(strings.TrimPrefix(value, prefix)); err != nil {
		return fmt.Errorf("%w: revision ID must contain a canonical SHA-256 digest", storage.ErrInvalid)
	}
	return nil
}

func validateRevisionFiles(files []storage.RevisionFile) error {
	seen := make(map[string]string, len(files))
	for _, file := range files {
		if err := storage.ValidateSHA256(file.SHA256); err != nil {
			return err
		}
		if file.Path == "" || strings.Contains(file.Path, "\\") || path.IsAbs(file.Path) || path.Clean(file.Path) != file.Path || file.Path == "." || file.Path == ".." || strings.HasPrefix(file.Path, "../") {
			return fmt.Errorf("%w: revision path %q is not a canonical relative path", storage.ErrInvalid, file.Path)
		}
		folded := strings.ToLower(file.Path)
		if previous, ok := seen[folded]; ok {
			return fmt.Errorf("%w: revision path collision between %q and %q", storage.ErrInvalid, previous, file.Path)
		}
		seen[folded] = file.Path
	}
	return nil
}

func mkdirPrivate(directory string) error {
	if err := os.MkdirAll(directory, privateDir); err != nil {
		return err
	}
	return os.Chmod(directory, privateDir)
}

func makeTreeReadOnly(root string) error {
	var directories []string
	if err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			directories = append(directories, current)
			return nil
		}
		return os.Chmod(current, readOnlyFile)
	}); err != nil {
		return err
	}
	sort.Slice(directories, func(i, j int) bool { return len(directories[i]) > len(directories[j]) })
	for _, directory := range directories {
		if err := os.Chmod(directory, readOnlyDir); err != nil {
			return err
		}
	}
	return nil
}

func makeTreeWritable(root string) error {
	if root == "" {
		return nil
	}
	return filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.Chmod(current, privateDir)
		}
		return nil
	})
}

func syncDirectory(directory string) error {
	file, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

var _ storage.BlobInventory = (*Store)(nil)
