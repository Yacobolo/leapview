package ocirunner

import (
	"context"
	"errors"
	"io"
	"os/exec"
)

type Spec struct {
	Name             string
	Image            string
	CPUs             string
	Memory           string
	ActionDirectory  string
	WorkingDirectory string
	Command          []string
}

func DockerArgs(spec Spec) []string {
	data := spec.ActionDirectory + "/data"
	args := []string{
		"run", "--rm", "--init", "--name", spec.Name,
		"--label", "rtest.action=" + spec.Name,
		"--network", "host",
		"--cpus", fallback(spec.CPUs, "1.5"), "--memory", fallback(spec.Memory, "2500m"),
		"--shm-size", "1g",
		"--pids-limit", "2048", "--stop-timeout", "10",
		"-v", spec.ActionDirectory + ":" + spec.ActionDirectory,
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", "rtest-go-build-cache:/root/.cache/go-build",
		"-v", "rtest-go-mod-cache:/go/pkg/mod",
		"-e", "TESTCONTAINERS_HOST_OVERRIDE=localhost",
		"-e", "TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock",
		"-e", "RYUK_RECONNECTION_TIMEOUT=5s",
		"-e", "RTEST_JOB_ID=" + spec.Name,
		"-e", "RTEST_JOB_ROOT=" + spec.ActionDirectory,
		"-e", "TMPDIR=/tmp",
		"-e", "TEST_DATA_DIR=" + data,
		"-w", spec.WorkingDirectory,
		fallback(spec.Image, "rtest-runner-standard:local"),
	}
	return append(args, spec.Command...)
}

func Run(ctx context.Context, docker string, spec Spec, stdout, stderr io.Writer) (int, error) {
	docker = fallback(docker, "docker")
	defer remove(docker, spec.Name)
	command := exec.CommandContext(ctx, docker, DockerArgs(spec)...)
	command.Stdout, command.Stderr = stdout, stderr
	err := command.Run()
	if err == nil {
		return 0, nil
	}
	if ctx.Err() != nil {
		return 1, ctx.Err()
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return 1, err
}

func remove(docker, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	_ = exec.CommandContext(ctx, docker, "rm", "-f", name).Run()
}

func fallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
