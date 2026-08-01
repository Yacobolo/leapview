package swarm

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/flidai/leapview/rtest/internal/protocol"
)

func TestCreateArgsUseReplicatedJobAndSamePathWorkspace(t *testing.T) {
	spec := Spec{
		ID: "rtest-job-1", Repository: "example/service", Suite: "integration", Runner: "standard",
		Image: "runner:test", CASAddress: "127.0.0.1:50051", CASInstance: "rtest",
		RootDigest: "abc/123", JobsRoot: "/var/lib/rtest/jobs", Command: []string{"go", "test", "./..."},
		Timeout: 15 * time.Minute, CPUs: "2", Memory: "4g",
		WorkingDirectory: "cmd/service", Environment: map[string]string{"RTEST_PROOF": "generic-oci"},
		EntrypointHostPath: "/usr/local/lib/rtest/rtest-job-entrypoint",
		CacheRoot:          "/var/lib/rtest/cache", ProjectID: "prj-example",
		Caches: []CacheMount{{Name: "go-build", Target: "/root/.cache/go-build"}, {Name: "modules", Target: "/go/pkg/mod"}},
	}
	args := CreateArgs(spec)
	for _, sequence := range [][]string{
		{"--mode", "replicated-job"},
		{"--restart-condition", "none"},
		{"--network", "host"},
		{"--reserve-cpu", "2"},
		{"--reserve-memory", "4g"},
		{"--mount", "type=bind,src=/var/lib/rtest/jobs,dst=/var/lib/rtest/jobs"},
		{"--mount", "type=bind,src=/var/lib/rtest/cache/prj-example/go-build,dst=/root/.cache/go-build"},
		{"--mount", "type=bind,src=/var/lib/rtest/cache/prj-example/modules,dst=/go/pkg/mod"},
		{"--mount", "type=bind,src=/usr/local/lib/rtest/rtest-job-entrypoint,dst=/usr/local/bin/rtest-job-entrypoint,readonly"},
		{"--label", "rtest.project=prj-example"},
		{"--label", "rtest.job=rtest-job-1"},
		{"--env", "RTEST_WORKSPACE=/var/lib/rtest/jobs/rtest-job-1/workspace"},
		{"--env", "RTEST_WORKER_LOCK=/var/lib/rtest/jobs/.worker.lock"},
		{"--env", "RTEST_ROOT_DIGEST=abc/123"},
		{"--env", "RTEST_WORKING_DIRECTORY=cmd/service"},
		{"--env", "RTEST_PROOF=generic-oci"},
		{"--entrypoint", "/usr/local/bin/rtest-job-entrypoint"},
		{"runner:test", "go", "test", "./..."},
	} {
		if !containsSequence(args, sequence) {
			t.Fatalf("args %#v missing sequence %#v", args, sequence)
		}
	}
	for _, forbidden := range []string{"rtest-go-build-cache", "rtest-go-mod-cache"} {
		if strings.Contains(strings.Join(args, "\n"), forbidden) {
			t.Fatalf("args %#v contain global cache %q", args, forbidden)
		}
	}
}

func TestTaskStatusMapsDockerLifecycle(t *testing.T) {
	tests := []struct {
		name      string
		state     string
		exitCode  int
		cancelled bool
		want      protocol.Status
	}{
		{name: "pending", state: "pending", want: protocol.StatusQueued},
		{name: "running", state: "running", want: protocol.StatusRunning},
		{name: "success", state: "complete", want: protocol.StatusSucceeded},
		{name: "failure", state: "failed", exitCode: 2, want: protocol.StatusFailed},
		{name: "timeout", state: "failed", exitCode: 124, want: protocol.StatusTimedOut},
		{name: "cancel", state: "shutdown", cancelled: true, want: protocol.StatusCancelled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := taskStatus(test.state, test.exitCode, test.cancelled); got != test.want {
				t.Fatalf("taskStatus() = %q, want %q", got, test.want)
			}
		})
	}
}

func containsSequence(values, sequence []string) bool {
	for i := 0; i+len(sequence) <= len(values); i++ {
		if slices.Equal(values[i:i+len(sequence)], sequence) {
			return true
		}
	}
	return false
}
