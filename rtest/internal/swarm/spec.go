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
	managedLabel             = "rtest.managed"
	cancelledLabel           = "rtest.cancelled"
	defaultSharedMemoryBytes = "1073741824"
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
	CacheRoot          string
	ProjectID          string
	Caches             []CacheMount
	HostUID            string
	HostGID            string
}

type CacheMount struct {
	Name   string
	Target string
}

func CreateArgs(spec Spec) []string {
	workspace := filepath.Join(spec.JobsRoot, spec.ID, "workspace")
	args := []string{
		"service", "create", "--detach", "--quiet", "--name", spec.ID, "--init",
		"--label", managedLabel + "=true",
		"--label", "rtest.project=" + spec.ProjectID,
		"--label", "rtest.job=" + spec.ID,
		"--label", "rtest.repository=" + encodeLabel(spec.Repository),
		"--label", "rtest.suite=" + encodeLabel(spec.Suite),
		"--label", "rtest.runner=" + encodeLabel(spec.Runner),
		"--label", "rtest.timeout_seconds=" + strconv.Itoa(int(spec.Timeout.Seconds())),
		"--label", "rtest.root_digest=" + spec.RootDigest,
		"--mode", "replicated-job", "--replicas", "1", "--max-concurrent", "1",
		"--restart-condition", "none", "--stop-grace-period", "15s",
		"--network", "host",
		"--user", "0:0",
		"--limit-cpu", fallback(spec.CPUs, "1.5"), "--limit-memory", fallback(spec.Memory, "2500m"),
		"--reserve-cpu", fallback(spec.CPUs, "1.5"), "--reserve-memory", fallback(spec.Memory, "2500m"),
		"--mount", "type=bind,src=" + spec.JobsRoot + ",dst=" + spec.JobsRoot,
		"--mount", "type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock",
		"--mount", "type=tmpfs,dst=/dev/shm,tmpfs-size=" + defaultSharedMemoryBytes,
		"--env", "RTEST_JOB_ID=" + spec.ID,
		"--env", "RTEST_WORKSPACE=" + workspace,
		"--env", "RTEST_WORKER_LOCK=" + filepath.Join(spec.JobsRoot, ".worker.lock"),
		"--env", "RTEST_HOST_UID=" + spec.HostUID,
		"--env", "RTEST_HOST_GID=" + spec.HostGID,
		"--env", "RTEST_CAS_ADDRESS=" + spec.CASAddress,
		"--env", "RTEST_CAS_INSTANCE=" + fallback(spec.CASInstance, "rtest"),
		"--env", "RTEST_ROOT_DIGEST=" + spec.RootDigest,
		"--env", "RTEST_TIMEOUT_MILLIS=" + strconv.FormatInt(spec.Timeout.Milliseconds(), 10),
		"--env", "RTEST_WORKING_DIRECTORY=" + fallback(spec.WorkingDirectory, "."),
		"--env", "TESTCONTAINERS_HOST_OVERRIDE=localhost",
		"--env", "TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock",
		"--env", "RYUK_RECONNECTION_TIMEOUT=5s",
		"--env", "TMPDIR=/tmp",
		"--env", "TEST_DATA_DIR=" + filepath.Join(workspace, ".rtest", "data"),
		"--entrypoint", "/usr/local/bin/rtest-job-entrypoint",
	}
	caches := append([]CacheMount(nil), spec.Caches...)
	sort.Slice(caches, func(i, j int) bool {
		if caches[i].Name == caches[j].Name {
			return caches[i].Target < caches[j].Target
		}
		return caches[i].Name < caches[j].Name
	})
	for _, cache := range caches {
		source := filepath.Join(spec.CacheRoot, spec.ProjectID, cache.Name)
		args = append(args, "--mount", "type=bind,src="+source+",dst="+cache.Target)
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
