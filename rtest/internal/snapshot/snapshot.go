package snapshot

import (
	"archive/tar"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

var ErrWorktreeChanged = errors.New("worktree changed while snapshot was being created")

type Result struct {
	Root   string
	Digest string
	Size   int64
	Files  int
}

type countingHashWriter struct {
	dst  io.Writer
	hash hash.Hash
	size int64
}

func (w *countingHashWriter) Write(p []byte) (int, error) {
	n, err := w.dst.Write(p)
	if n > 0 {
		_, _ = w.hash.Write(p[:n])
		w.size += int64(n)
	}
	return n, err
}

type metadata struct {
	mode    os.FileMode
	size    int64
	modTime time.Time
	link    string
}

func Create(ctx context.Context, directory string, dst io.Writer) (Result, error) {
	root, err := gitOutput(ctx, directory, "rev-parse", "--show-toplevel")
	if err != nil {
		return Result{}, fmt.Errorf("resolve Git worktree: %w", err)
	}
	root = strings.TrimSpace(root)
	pathsRaw, err := gitBytes(ctx, root, "ls-files", "--cached", "--others", "--exclude-standard", "--deduplicate", "-z")
	if err != nil {
		return Result{}, fmt.Errorf("list worktree files: %w", err)
	}
	paths := splitNUL(pathsRaw)
	sort.Strings(paths)

	checksum := sha256.New()
	output := &countingHashWriter{dst: dst, hash: checksum}
	encoder, err := zstd.NewWriter(output, zstd.WithEncoderConcurrency(1), zstd.WithEncoderLevel(zstd.SpeedFastest))
	if err != nil {
		return Result{}, fmt.Errorf("create zstd encoder: %w", err)
	}
	tarWriter := tar.NewWriter(encoder)
	written := 0
	for _, name := range paths {
		if err := validateArchivePath(name); err != nil {
			_ = tarWriter.Close()
			_ = encoder.Close()
			return Result{}, err
		}
		path := filepath.Join(root, filepath.FromSlash(name))
		before, err := inspect(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return Result{}, fmt.Errorf("inspect %q: %w", name, err)
		}
		header := &tar.Header{
			Name:       filepath.ToSlash(name),
			Mode:       int64(before.mode.Perm()),
			Uid:        0,
			Gid:        0,
			ModTime:    time.Unix(0, 0).UTC(),
			AccessTime: time.Time{},
			ChangeTime: time.Time{},
			Format:     tar.FormatPAX,
		}
		switch {
		case before.mode.IsRegular():
			header.Typeflag = tar.TypeReg
			header.Size = before.size
			if err := tarWriter.WriteHeader(header); err != nil {
				return Result{}, fmt.Errorf("write header %q: %w", name, err)
			}
			file, err := os.Open(path)
			if err != nil {
				return Result{}, fmt.Errorf("open %q: %w", name, err)
			}
			_, copyErr := io.CopyN(tarWriter, file, before.size)
			closeErr := file.Close()
			if copyErr != nil {
				return Result{}, fmt.Errorf("archive %q: %w", name, copyErr)
			}
			if closeErr != nil {
				return Result{}, fmt.Errorf("close %q: %w", name, closeErr)
			}
		case before.mode&os.ModeSymlink != 0:
			header.Typeflag = tar.TypeSymlink
			header.Linkname = before.link
			if err := tarWriter.WriteHeader(header); err != nil {
				return Result{}, fmt.Errorf("write symlink %q: %w", name, err)
			}
		default:
			return Result{}, fmt.Errorf("unsupported worktree entry %q with mode %s", name, before.mode)
		}
		after, err := inspect(path)
		if err != nil || before != after {
			return Result{}, fmt.Errorf("%w: %s", ErrWorktreeChanged, name)
		}
		written++
	}
	if err := tarWriter.Close(); err != nil {
		return Result{}, fmt.Errorf("close tar archive: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return Result{}, fmt.Errorf("close zstd archive: %w", err)
	}
	return Result{
		Root:   root,
		Digest: "sha256:" + hex.EncodeToString(checksum.Sum(nil)),
		Size:   output.size,
		Files:  written,
	}, nil
}

func inspect(path string) (metadata, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return metadata{}, err
	}
	value := metadata{mode: info.Mode(), size: info.Size(), modTime: info.ModTime()}
	if info.Mode()&os.ModeSymlink != 0 {
		value.link, err = os.Readlink(path)
	}
	return value, err
}

func validateArchivePath(name string) error {
	clean := filepath.ToSlash(filepath.Clean(name))
	if name == "" || filepath.IsAbs(name) || clean == ".." || strings.HasPrefix(clean, "../") || strings.ContainsRune(name, '\x00') {
		return fmt.Errorf("unsafe worktree path %q", name)
	}
	return nil
}

func splitNUL(data []byte) []string {
	result := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Split(splitAtNUL)
	for scanner.Scan() {
		if scanner.Text() != "" {
			result = append(result, scanner.Text())
		}
	}
	return result
}

func splitAtNUL(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i, b := range data {
		if b == 0 {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	data, err := gitBytes(ctx, dir, args...)
	return string(data), err
}

func gitBytes(ctx context.Context, dir string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = dir
	output, err := command.Output()
	if err == nil {
		return output, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(exitErr.Stderr)))
	}
	return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}
