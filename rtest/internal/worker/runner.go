package worker

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/flidai/leapview/rtest/internal/protocol"
	"github.com/klauspost/compress/zstd"
)

type Runner struct {
	Docker    string
	WorkRoot  string
	CacheRoot string
	Image     string
	CPUs      string
	Memory    string
}

func (r Runner) Run(parent context.Context, job protocol.Job, source io.Reader, output io.Writer) protocol.FinishRequest {
	jobRoot := filepath.Join(r.WorkRoot, job.ID)
	workspace := filepath.Join(jobRoot, "workspace")
	tmpDir := filepath.Join(jobRoot, "tmp")
	dataDir := filepath.Join(jobRoot, "data")
	cacheRoot := fallback(r.CacheRoot, filepath.Join(filepath.Dir(r.WorkRoot), "cache"))
	goBuildCache := filepath.Join(cacheRoot, "go-build")
	goModCache := filepath.Join(cacheRoot, "go-mod")
	for _, directory := range []string{workspace, tmpDir, dataDir, goBuildCache, goModCache} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return lost(err)
		}
	}
	if err := os.Chmod(jobRoot, 0o700); err != nil {
		return lost(err)
	}
	defer os.RemoveAll(jobRoot)
	sourceFile, err := storeAndVerifySource(source, filepath.Join(jobRoot, "source.tar.zst"), job.SourceDigest)
	if err != nil {
		return lost(err)
	}
	defer sourceFile.Close()
	if err := extract(sourceFile, workspace); err != nil {
		return lost(fmt.Errorf("extract source: %w", err))
	}
	timeout := time.Duration(max(1, job.TimeoutSeconds)) * time.Second
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	docker := r.Docker
	if docker == "" {
		docker = "docker"
	}
	containerName := "rtest-" + job.ID
	defer removeContainer(docker, containerName)
	args := []string{
		"run", "--rm", "--init", "--name", containerName,
		"--label", "rtest.job=" + job.ID,
		"--network", "host",
		"--cpus", fallback(r.CPUs, "1.5"), "--memory", fallback(r.Memory, "2500m"),
		"--pids-limit", "2048", "--stop-timeout", "10",
		"-v", workspace + ":" + workspace,
		"-v", tmpDir + ":" + tmpDir,
		"-v", dataDir + ":" + dataDir,
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", goBuildCache + ":/root/.cache/go-build",
		"-v", goModCache + ":/go/pkg/mod",
		"-e", "TESTCONTAINERS_HOST_OVERRIDE=localhost",
		"-e", "TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock",
		"-e", "RYUK_RECONNECTION_TIMEOUT=5s",
		"-e", "RTEST_JOB_ID=" + job.ID,
		"-e", "RTEST_JOB_ROOT=" + jobRoot,
		"-e", "TMPDIR=" + tmpDir,
		"-e", "TEST_DATA_DIR=" + dataDir,
		"-w", workspace, r.Image,
	}
	args = append(args, job.Command...)
	command := exec.CommandContext(ctx, docker, args...)
	command.Stdout, command.Stderr = output, output
	err = command.Run()
	if err == nil {
		code := 0
		return protocol.FinishRequest{Status: protocol.StatusSucceeded, ExitCode: &code}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return protocol.FinishRequest{Status: protocol.StatusTimedOut, ErrorMessage: ctx.Err().Error()}
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return protocol.FinishRequest{Status: protocol.StatusCancelled, ErrorMessage: ctx.Err().Error()}
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		code := exitError.ExitCode()
		return protocol.FinishRequest{Status: protocol.StatusFailed, ExitCode: &code, ErrorMessage: err.Error()}
	}
	return lost(err)
}

func storeAndVerifySource(source io.Reader, path, expected string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(file, hash), source); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("store source: %w", err)
	}
	actual := "sha256:" + hex.EncodeToString(hash.Sum(nil))
	if expected != "" && actual != expected {
		_ = file.Close()
		return nil, fmt.Errorf("source checksum mismatch: got %s, want %s", actual, expected)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func removeContainer(docker, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, docker, "rm", "-f", name).Run()
}

func extract(source io.Reader, destination string) error {
	decoder, err := zstd.NewReader(source)
	if err != nil {
		return err
	}
	defer decoder.Close()
	reader := tar.NewReader(decoder)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		clean := filepath.Clean(filepath.FromSlash(header.Name))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe archive path %q", header.Name)
		}
		path := filepath.Join(destination, clean)
		if !within(destination, path) {
			return fmt.Errorf("unsafe archive path %q", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(header.Mode)&0o777)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(file, reader, header.Size)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			linkTarget := filepath.Clean(filepath.FromSlash(header.Linkname))
			if filepath.IsAbs(linkTarget) || !within(destination, filepath.Join(filepath.Dir(path), linkTarget)) {
				return fmt.Errorf("unsafe symlink %q", header.Name)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(linkTarget, path); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported archive entry %q", header.Name)
		}
	}
}

func within(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func lost(err error) protocol.FinishRequest {
	return protocol.FinishRequest{Status: protocol.StatusLost, ErrorMessage: err.Error()}
}

func fallback(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}
