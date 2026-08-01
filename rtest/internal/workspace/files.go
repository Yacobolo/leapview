package workspace

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

// Files returns the tracked and untracked, non-ignored files that describe the
// current Git worktree. Deleted tracked paths are intentionally omitted.
func Files(ctx context.Context, root string) ([]string, error) {
	command := exec.CommandContext(ctx, "git", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("select worktree files: %w", err)
	}
	items := bytes.Split(output, []byte{0})
	files := make([]string, 0, len(items))
	for _, item := range items {
		if len(item) == 0 {
			continue
		}
		path := filepath.FromSlash(string(item))
		if _, err := os.Lstat(filepath.Join(root, path)); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("inspect worktree input %q: %w", path, err)
		}
		files = append(files, path)
	}
	sort.Strings(files)
	return files, nil
}
