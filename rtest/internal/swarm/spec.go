package swarm

import (
	"encoding/base64"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/flidai/leapview/rtest/internal/protocol"
)

const (
	managedLabel   = "rtest.managed"
	cancelledLabel = "rtest.cancelled"
)

type Spec struct {
	ID                 string
	Repository         string
	Suite              string
	Runner             string
	Image              string
	CASAddress         string
	CASInstance        string
	RootDigest         string
	JobsRoot           string
	Command            []string
	WorkingDirectory   string
	Environment        map[string]string
	EntrypointHostPath string
	Timeout            time.Duration
	CPUs               string
	Memory             string
}

func CreateArgs(spec Spec) []string {
	workspace := filepath.Join(spec.JobsRoot, spec.ID, "workspace")
	args := []string{
		"service", "create", "--detach", "--quiet", "--name", spec.ID, "--init",
		"--label", managedLabel + "=true",
		"--label", "rtest.repository=" + encodeLabel(spec.Repository),
		"--label", "rtest.suite=" + encodeLabel(spec.Suite),
		"--label", "rtest.runner=" + encodeLabel(spec.Runner),
		"--label", "rtest.timeout_seconds=" + strconv.Itoa(int(spec.Timeout.Seconds())),
		"--label", "rtest.root_digest=" + spec.RootDigest,
		"--mode", "replicated-job", "--replicas", "1", "--max-concurrent", "1",
		"--restart-condition", "none", "--stop-grace-period", "15s",
		"--network", "host",
		"--limit-cpu", fallback(spec.CPUs, "1.5"), "--limit-memory", fallback(spec.Memory, "2500m"),
		"--reserve-cpu", fallback(spec.CPUs, "1.5"), "--reserve-memory", fallback(spec.Memory, "2500m"),
		"--mount", "type=bind,src=" + spec.JobsRoot + ",dst=" + spec.JobsRoot,
		"--mount", "type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock",
		"--mount", "type=volume,src=rtest-go-build-cache,dst=/root/.cache/go-build",
		"--mount", "type=volume,src=rtest-go-mod-cache,dst=/go/pkg/mod",
		"--env", "RTEST_JOB_ID=" + spec.ID,
		"--env", "RTEST_WORKSPACE=" + workspace,
		"--env", "RTEST_CAS_ADDRESS=" + spec.CASAddress,
		"--env", "RTEST_CAS_INSTANCE=" + fallback(spec.CASInstance, "rtest"),
		"--env", "RTEST_ROOT_DIGEST=" + spec.RootDigest,
		"--env", "RTEST_TIMEOUT_MILLIS=" + strconv.FormatInt(spec.Timeout.Milliseconds(), 10),
		"--env", "RTEST_WORKING_DIRECTORY=" + fallback(spec.WorkingDirectory, "."),
		"--env", "TESTCONTAINERS_HOST_OVERRIDE=localhost",
		"--env", "TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock",
		"--env", "RYUK_RECONNECTION_TIMEOUT=5s",
		"--env", "TMPDIR=" + filepath.Join(spec.JobsRoot, spec.ID, "tmp"),
		"--env", "TEST_DATA_DIR=" + filepath.Join(workspace, ".rtest", "data"),
		"--entrypoint", "/usr/local/bin/rtest-job-entrypoint",
	}
	if spec.EntrypointHostPath != "" {
		args = append(args, "--mount", "type=bind,src="+spec.EntrypointHostPath+",dst=/usr/local/bin/rtest-job-entrypoint,readonly")
	}
	keys := make([]string, 0, len(spec.Environment))
	for key := range spec.Environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "--env", key+"="+spec.Environment[key])
	}
	args = append(args, fallback(spec.Image, "rtest-runner-standard:local"))
	return append(args, spec.Command...)
}

func taskStatus(state string, exitCode int, cancelled bool) protocol.Status {
	if cancelled {
		return protocol.StatusCancelled
	}
	switch state {
	case "complete":
		if exitCode == 0 {
			return protocol.StatusSucceeded
		}
		return protocol.StatusFailed
	case "failed", "rejected", "orphaned":
		if exitCode == 124 {
			return protocol.StatusTimedOut
		}
		return protocol.StatusFailed
	case "running":
		return protocol.StatusRunning
	default:
		return protocol.StatusQueued
	}
}

func encodeLabel(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodeLabel(value string) string {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return value
	}
	return string(decoded)
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
