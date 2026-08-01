package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const FileName = ".rtest.json"

type Suite struct {
	Command        []string `json:"command"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
}

type File struct {
	Repository string           `json:"repository"`
	Runner     string           `json:"runner,omitempty"`
	Suites     map[string]Suite `json:"suites"`
}

type Resolved struct {
	Repository     string
	Runner         string
	Suite          string
	Command        []string
	TimeoutSeconds int
}

func Root(ctx context.Context, directory string) (string, error) {
	command := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	command.Dir = directory
	data, err := command.Output()
	if err != nil {
		return "", errors.New("current directory is not inside a Git worktree")
	}
	return strings.TrimSpace(string(data)), nil
}

func Load(root, suiteName string, extra []string) (Resolved, error) {
	config, err := read(root)
	if err != nil {
		return Resolved{}, err
	}
	suite, ok := config.Suites[suiteName]
	if !ok {
		return Resolved{}, fmt.Errorf("suite %q is not defined in %s", suiteName, FileName)
	}
	if len(suite.Command) == 0 {
		return Resolved{}, fmt.Errorf("suite %q has an empty command", suiteName)
	}
	timeout := suite.TimeoutSeconds
	if timeout == 0 {
		timeout = 1800
	}
	if timeout < 1 || timeout > 3600 {
		return Resolved{}, fmt.Errorf("suite %q timeout must be between 1 and 3600 seconds", suiteName)
	}
	return Resolved{
		Repository:     config.Repository,
		Runner:         runner(config.Runner),
		Suite:          suiteName,
		Command:        append(append([]string(nil), suite.Command...), extra...),
		TimeoutSeconds: timeout,
	}, nil
}

func Command(root string, command []string, timeoutSeconds int) (Resolved, error) {
	if len(command) == 0 {
		return Resolved{}, errors.New("command is required")
	}
	config, err := read(root)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return Resolved{}, err
		}
		config = File{Repository: filepath.Base(root)}
	}
	if timeoutSeconds < 1 || timeoutSeconds > 3600 {
		return Resolved{}, errors.New("timeout must be between 1 and 3600 seconds")
	}
	return Resolved{
		Repository:     config.Repository,
		Runner:         runner(config.Runner),
		Suite:          "command",
		Command:        append([]string(nil), command...),
		TimeoutSeconds: timeoutSeconds,
	}, nil
}

func read(root string) (File, error) {
	file, err := os.Open(filepath.Join(root, FileName))
	if err != nil {
		return File{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var config File
	if err := decoder.Decode(&config); err != nil {
		return File{}, fmt.Errorf("read %s: %w", FileName, err)
	}
	if config.Repository == "" {
		return File{}, fmt.Errorf("%s: repository is required", FileName)
	}
	if config.Runner != "" && config.Runner != "standard" {
		return File{}, fmt.Errorf("%s: unsupported runner %q", FileName, config.Runner)
	}
	return config, nil
}

func runner(value string) string {
	if value == "" {
		return "standard"
	}
	return value
}
