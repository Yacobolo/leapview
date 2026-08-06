// Package sharedassets installs immutable development assets once in a user-level
// cache and leaves stable symlinks at the worktree-local paths expected by LeapView.
package sharedassets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const DefaultDirectoryName = "dev-assets"

type Options struct {
	Local    string
	Shared   string
	Ready    func(string) error
	Populate func(string) error
}

func CacheRoot(override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		root, err := filepath.Abs(override)
		if err != nil {
			return "", fmt.Errorf("resolve shared development asset cache %s: %w", override, err)
		}
		return root, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("find user cache directory: %w", err)
	}
	return filepath.Join(base, "leapview", DefaultDirectoryName), nil
}

func Ensure(options Options) error {
	if options.Ready == nil || options.Populate == nil {
		return fmt.Errorf("shared assets require readiness and population functions")
	}
	local, err := filepath.Abs(options.Local)
	if err != nil {
		return fmt.Errorf("resolve local asset directory %s: %w", options.Local, err)
	}
	shared, err := filepath.Abs(options.Shared)
	if err != nil {
		return fmt.Errorf("resolve shared asset directory %s: %w", options.Shared, err)
	}
	if local == shared || contains(local, shared) || contains(shared, local) {
		return fmt.Errorf("local and shared asset directories must be separate: %s and %s", local, shared)
	}

	if options.Ready(shared) == nil {
		return linkLocal(local, shared)
	}

	if info, statErr := os.Lstat(local); statErr == nil && info.IsDir() && options.Ready(local) == nil {
		if err := os.MkdirAll(filepath.Dir(shared), 0o755); err != nil {
			return fmt.Errorf("create shared asset parent: %w", err)
		}
		if _, sharedErr := os.Lstat(shared); os.IsNotExist(sharedErr) {
			if err := os.Rename(local, shared); err != nil {
				return fmt.Errorf("migrate assets from %s to %s: %w", local, shared, err)
			}
			if err := linkLocal(local, shared); err != nil {
				_ = os.Rename(shared, local)
				return err
			}
			return nil
		}
	}

	if err := populateShared(shared, options.Ready, options.Populate); err != nil {
		return err
	}
	return linkLocal(local, shared)
}

func populateShared(shared string, ready, populate func(string) error) error {
	if err := os.MkdirAll(filepath.Dir(shared), 0o755); err != nil {
		return fmt.Errorf("create shared asset parent: %w", err)
	}
	temporary, err := os.MkdirTemp(filepath.Dir(shared), "."+filepath.Base(shared)+"-install-")
	if err != nil {
		return fmt.Errorf("create temporary shared asset directory: %w", err)
	}
	defer os.RemoveAll(temporary)
	if err := populate(temporary); err != nil {
		return err
	}
	if err := ready(temporary); err != nil {
		return fmt.Errorf("verify populated shared assets: %w", err)
	}

	// Another worktree may have completed the same immutable package while this
	// one was being populated. Prefer that valid package and discard our staging
	// directory instead of replacing it.
	if ready(shared) == nil {
		return nil
	}
	backup, moved, err := moveAside(shared)
	if err != nil {
		return err
	}
	if err := os.Rename(temporary, shared); err != nil {
		if moved {
			_ = os.Rename(backup, shared)
		}
		return fmt.Errorf("publish shared assets to %s: %w", shared, err)
	}
	if moved {
		_ = os.RemoveAll(backup)
	}
	return nil
}

func linkLocal(local, shared string) error {
	info, err := os.Lstat(local)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
			return fmt.Errorf("create local asset parent: %w", err)
		}
		if err := os.Symlink(shared, local); err != nil {
			return fmt.Errorf("link %s to shared assets %s: %w", local, shared, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect local asset path %s: %w", local, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(local)
		if err != nil {
			return err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(local), target)
		}
		if filepath.Clean(target) == filepath.Clean(shared) {
			return nil
		}
		return fmt.Errorf("local asset path %s already links to %s, not %s", local, target, shared)
	}
	if !info.IsDir() {
		return fmt.Errorf("local asset path %s exists and is not a directory", local)
	}

	backup, moved, err := moveAside(local)
	if err != nil {
		return err
	}
	if !moved {
		return fmt.Errorf("could not move local asset directory %s aside", local)
	}
	if err := os.Symlink(shared, local); err != nil {
		_ = os.Rename(backup, local)
		return fmt.Errorf("link %s to shared assets %s: %w", local, shared, err)
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove replaced local asset directory %s: %w", backup, err)
	}
	return nil
}

func moveAside(target string) (string, bool, error) {
	if _, err := os.Lstat(target); os.IsNotExist(err) {
		return "", false, nil
	} else if err != nil {
		return "", false, fmt.Errorf("inspect asset directory %s: %w", target, err)
	}
	placeholder, err := os.MkdirTemp(filepath.Dir(target), "."+filepath.Base(target)+"-backup-")
	if err != nil {
		return "", false, fmt.Errorf("reserve asset backup path: %w", err)
	}
	if err := os.Remove(placeholder); err != nil {
		return "", false, err
	}
	if err := os.Rename(target, placeholder); err != nil {
		return "", false, fmt.Errorf("move asset directory %s aside: %w", target, err)
	}
	return placeholder, true, nil
}

func contains(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
