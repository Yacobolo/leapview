package recovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Snapshotter interface {
	Backup(context.Context, string) error
}

type manifest struct {
	Format    int               `json:"format"`
	CreatedAt time.Time         `json:"created_at"`
	Files     map[string]string `json:"files"`
}

func Create(ctx context.Context, snapshotter Snapshotter, dataDir, output string) error {
	if snapshotter == nil || dataDir == "" || output == "" {
		return errors.New("snapshotter, data directory, and output are required")
	}
	if _, err := os.Stat(output); err == nil {
		return errors.New("backup output already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	parent := filepath.Dir(output)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	temporary, err := os.MkdirTemp(parent, ".rtest-backup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	if err := os.Chmod(temporary, 0o700); err != nil {
		return err
	}
	database := filepath.Join(temporary, "control", "control.db")
	if err := os.MkdirAll(filepath.Dir(database), 0o700); err != nil {
		return err
	}
	if err := snapshotter.Backup(ctx, database); err != nil {
		return err
	}
	if err := copyRegular(filepath.Join(dataDir, "control", "token-pepper"), filepath.Join(temporary, "control", "token-pepper")); err != nil {
		return fmt.Errorf("copy token pepper: %w", err)
	}
	if err := copyTree(filepath.Join(dataDir, "pki"), filepath.Join(temporary, "pki")); err != nil {
		return fmt.Errorf("copy PKI: %w", err)
	}

	files, err := hashes(temporary)
	if err != nil {
		return err
	}
	for _, required := range []string{"control/control.db", "control/token-pepper", "pki/ca.pem", "pki/ca-key.pem"} {
		if _, ok := files[required]; !ok {
			return fmt.Errorf("backup source is missing %s", required)
		}
	}
	data, err := json.MarshalIndent(manifest{Format: 1, CreatedAt: time.Now().UTC(), Files: files}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(temporary, "manifest.json"), data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, output)
}

func Restore(input, dataDir string) error {
	if input == "" || dataDir == "" {
		return errors.New("backup input and data directory are required")
	}
	if _, err := os.Stat(dataDir); err == nil {
		return errors.New("restore data directory already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := os.ReadFile(filepath.Join(input, "manifest.json"))
	if err != nil {
		return err
	}
	var manifest manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("decode backup manifest: %w", err)
	}
	if manifest.Format != 1 || len(manifest.Files) == 0 {
		return errors.New("unsupported or empty backup manifest")
	}
	for name, want := range manifest.Files {
		if !safeRelative(name) {
			return fmt.Errorf("unsafe backup path %q", name)
		}
		got, err := hashFile(filepath.Join(input, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("backup checksum mismatch for %s", name)
		}
	}
	for _, required := range []string{"control/control.db", "control/token-pepper", "pki/ca.pem", "pki/ca-key.pem"} {
		if _, ok := manifest.Files[required]; !ok {
			return fmt.Errorf("backup is missing %s", required)
		}
	}
	parent := filepath.Dir(dataDir)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	temporary, err := os.MkdirTemp(parent, ".rtest-restore-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	if err := os.Chmod(temporary, 0o700); err != nil {
		return err
	}
	names := make([]string, 0, len(manifest.Files))
	for name := range manifest.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := copyRegular(filepath.Join(input, filepath.FromSlash(name)), filepath.Join(temporary, filepath.FromSlash(name))); err != nil {
			return err
		}
	}
	return os.Rename(temporary, dataDir)
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("backup source contains non-regular file %s", path)
		}
		return copyRegular(path, target)
	})
}

func copyRegular(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", source)
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func hashes(root string) (map[string]string, error) {
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("backup contains non-regular file %s", path)
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(name)], err = hashFile(path)
		return err
	})
	return result, err
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func safeRelative(name string) bool {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
	return name != "" && clean == name && !filepath.IsAbs(name) && name != ".." && !strings.HasPrefix(name, "../")
}
