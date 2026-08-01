package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/flidai/leapview/rtest/internal/cas"
)

type result struct {
	Status     string    `json:"status"`
	ExitCode   int       `json:"exit_code"`
	FinishedAt time.Time `json:"finished_at"`
}

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "rtest-job-entrypoint: command is required")
		return 2
	}
	workspace := os.Getenv("RTEST_WORKSPACE")
	if workspace == "" {
		fmt.Fprintln(os.Stderr, "rtest-job-entrypoint: RTEST_WORKSPACE is required")
		return 2
	}
	jobDirectory := filepath.Dir(workspace)
	if err := os.MkdirAll(jobDirectory, 0o700); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.MkdirAll(filepath.Join(jobDirectory, "tmp"), 0o700); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	logFile, err := os.OpenFile(filepath.Join(jobDirectory, "job.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer logFile.Close()
	stdout := io.MultiWriter(os.Stdout, logFile)
	stderr := io.MultiWriter(os.Stderr, logFile)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if timeout := timeoutFromEnvironment(); timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if err := os.RemoveAll(workspace); err != nil {
		fmt.Fprintln(stderr, err)
		return finish(jobDirectory, "failed", 1)
	}
	if err := cas.Materialize(ctx, required("RTEST_CAS_ADDRESS"), fallback(os.Getenv("RTEST_CAS_INSTANCE"), "rtest"), required("RTEST_ROOT_DIGEST"), workspace); err != nil {
		fmt.Fprintln(stderr, err)
		return finish(jobDirectory, "failed", 1)
	}
	for _, directory := range []string{filepath.Join(workspace, ".rtest", "tmp"), filepath.Join(workspace, ".rtest", "data")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			fmt.Fprintln(stderr, err)
			return finish(jobDirectory, "failed", 1)
		}
	}
	command := exec.CommandContext(ctx, os.Args[1], os.Args[2:]...)
	workingDirectory, err := resolveWorkingDirectory(workspace, os.Getenv("RTEST_WORKING_DIRECTORY"))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return finish(jobDirectory, "failed", 2)
	}
	command.Dir, command.Stdout, command.Stderr = workingDirectory, stdout, stderr
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		return syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
	}
	command.WaitDelay = 5 * time.Second
	err = command.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		fmt.Fprintln(stderr, "remote job timed out")
		return finish(jobDirectory, "timed_out", 124)
	}
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "remote job cancelled")
		return finish(jobDirectory, "cancelled", 130)
	}
	if err == nil {
		return finish(jobDirectory, "succeeded", 0)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return finish(jobDirectory, "failed", exit.ExitCode())
	}
	fmt.Fprintln(stderr, err)
	return finish(jobDirectory, "failed", 1)
}

func resolveWorkingDirectory(workspace, relative string) (string, error) {
	if relative == "" || relative == "." {
		return workspace, nil
	}
	if filepath.IsAbs(relative) {
		return "", errors.New("RTEST_WORKING_DIRECTORY must be relative to the workspace")
	}
	clean := filepath.Clean(relative)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("RTEST_WORKING_DIRECTORY escapes the workspace")
	}
	return filepath.Join(workspace, clean), nil
}

func timeoutFromEnvironment() time.Duration {
	millis, err := strconv.ParseInt(os.Getenv("RTEST_TIMEOUT_MILLIS"), 10, 64)
	if err != nil || millis < 1 {
		return 0
	}
	return time.Duration(millis) * time.Millisecond
}

func finish(jobDirectory, status string, exitCode int) int {
	path := filepath.Join(jobDirectory, "result.json")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err == nil {
		_ = json.NewEncoder(file).Encode(result{Status: status, ExitCode: exitCode, FinishedAt: time.Now().UTC()})
		_ = file.Close()
	}
	return exitCode
}

func required(name string) string {
	value := os.Getenv(name)
	if value == "" {
		fmt.Fprintf(os.Stderr, "rtest-job-entrypoint: %s is required\n", name)
	}
	return value
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
