package ocirunner_test

import (
	"strings"
	"testing"

	"github.com/flidai/leapview/rtest/internal/ocirunner"
)

func TestDockerArgsPreserveActionPathsAndTestcontainersContract(t *testing.T) {
	got := strings.Join(ocirunner.DockerArgs(ocirunner.Spec{
		Name: "rtest-reapi-42", Image: "runner@sha256:abc", CPUs: "2", Memory: "3g",
		ActionDirectory: "/var/lib/rtest/reapi/work/action-1", WorkingDirectory: "/var/lib/rtest/reapi/work/action-1/work",
		Command: []string{"go", "test", "./..."},
	}), "\n")
	for _, required := range []string{
		"--name\nrtest-reapi-42", "--network\nhost", "--cpus\n2", "--memory\n3g", "--shm-size\n1g",
		"/var/run/docker.sock:/var/run/docker.sock",
		"/var/lib/rtest/reapi/work/action-1:/var/lib/rtest/reapi/work/action-1",
		"TESTCONTAINERS_HOST_OVERRIDE=localhost", "TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock",
		"TMPDIR=/tmp",
		"rtest-go-build-cache:/root/.cache/go-build", "rtest-go-mod-cache:/go/pkg/mod",
		"-w\n/var/lib/rtest/reapi/work/action-1/work", "runner@sha256:abc\ngo\ntest\n./...",
	} {
		if !strings.Contains(got, required) {
			t.Fatalf("docker arguments missing %q:\n%s", required, got)
		}
	}
}
