package ocirunner_test

import (
	"os"
	"strings"
	"testing"
)

func TestStandardRunnerUsesCurrentPinnedDockerCLI(t *testing.T) {
	data, err := os.ReadFile("../../runner/standard/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	dockerfile := string(data)
	for _, required := range []string{
		"FROM docker:29.1.3-cli@sha256:4fa0ee1f3a7e4354c4ea34558b6d4ee32859baf4973d4c8ccc8e7fe3dd730c04 AS docker-cli",
		"COPY --from=docker-cli /usr/local/bin/docker /usr/local/bin/docker",
	} {
		if !strings.Contains(dockerfile, required) {
			t.Fatalf("standard runner Dockerfile missing %q", required)
		}
	}
	if strings.Contains(dockerfile, "ca-certificates docker.io git") {
		t.Fatal("standard runner must not install the stale Debian docker.io CLI")
	}
}
