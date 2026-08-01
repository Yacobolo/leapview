// Package projectlink resolves the non-secret rtest project selected by a Git repository.
package projectlink

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const FileName = "rtest.json"

type File struct {
	Project string `json:"project"`
}

var projectPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// Resolve applies the public precedence contract: explicit flag, environment,
// then the nearest repository link while walking toward the Git root.
func Resolve(ctx context.Context, directory, explicit, environment string) (string, error) {
	if explicit != "" {
		return validateProject(explicit)
	}
	if environment != "" {
		return validateProject(environment)
	}
	root, err := repositoryRoot(ctx, directory)
	if err != nil {
		return "", err
	}
	current, err := canonicalPath(directory)
	if err != nil {
		return "", err
	}
	root, err = canonicalPath(root)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, current)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("current directory is outside the Git worktree")
	}
	for {
		path := filepath.Join(current, FileName)
		linked, err := read(path)
		if err == nil {
			return linked.Project, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if current == root {
			break
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", fmt.Errorf("project is required: pass --project, set RTEST_PROJECT, or run rtest init to create %s", FileName)
}

func canonicalPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absolute)
}

// Write creates an idempotent, commit-safe project link in directory. It does
// not overwrite a link to a different project.
func Write(directory, project string) (string, error) {
	project, err := validateProject(project)
	if err != nil {
		return "", err
	}
	path := filepath.Join(directory, FileName)
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%s must not be a symbolic link", path)
		}
		existing, err := read(path)
		if err != nil {
			return "", err
		}
		if existing.Project != project {
			return "", fmt.Errorf("%s already links project %q", path, existing.Project)
		}
		return path, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	payload, err := json.MarshalIndent(File{Project: project}, "", "  ")
	if err != nil {
		return "", err
	}
	payload = append(payload, '\n')
	temporary, err := os.CreateTemp(directory, ".rtest-link-*")
	if err != nil {
		return "", err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if _, err := temporary.Write(payload); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return "", err
	}
	return path, nil
}

func read(path string) (File, error) {
	file, err := os.Open(path)
	if err != nil {
		return File{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return File{}, err
	}
	if info.Size() > 4096 {
		return File{}, fmt.Errorf("read %s: repository link exceeds 4096 bytes", path)
	}
	decoder := json.NewDecoder(file)
	var linked File
	token, err := decoder.Token()
	if err != nil {
		return File{}, fmt.Errorf("read %s: %w", path, err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return File{}, fmt.Errorf("read %s: expected a JSON object", path)
	}
	seenProject := false
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return File{}, fmt.Errorf("read %s: %w", path, err)
		}
		key, ok := token.(string)
		if !ok {
			return File{}, fmt.Errorf("read %s: expected a field name", path)
		}
		if key != "project" {
			return File{}, fmt.Errorf("read %s: unknown field %q", path, key)
		}
		if seenProject {
			return File{}, fmt.Errorf("read %s: duplicate project field", path)
		}
		seenProject = true
		if err := decoder.Decode(&linked.Project); err != nil {
			return File{}, fmt.Errorf("read %s: %w", path, err)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return File{}, fmt.Errorf("read %s: %w", path, err)
	}
	if err := ensureEOF(decoder); err != nil {
		return File{}, fmt.Errorf("read %s: %w", path, err)
	}
	linked.Project, err = validateProject(linked.Project)
	if err != nil {
		return File{}, fmt.Errorf("read %s: %w", path, err)
	}
	return linked, nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("multiple JSON values")
}

func validateProject(project string) (string, error) {
	if !projectPattern.MatchString(project) {
		return "", errors.New("project must contain 1 to 128 letters, digits, dots, underscores, or hyphens")
	}
	return project, nil
}

func repositoryRoot(ctx context.Context, directory string) (string, error) {
	command := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	command.Dir = directory
	output, err := command.Output()
	if err != nil {
		return "", errors.New("current directory is not inside a Git worktree")
	}
	return strings.TrimSpace(string(output)), nil
}
