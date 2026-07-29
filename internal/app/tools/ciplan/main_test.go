package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	platformci "github.com/flidai/leapview/internal/platform/ci"
)

func TestWriteGitHubOutputsUsesNonEmptySentinelMatrices(t *testing.T) {
	t.Parallel()

	output := filepath.Join(t.TempDir(), "github-output")
	if err := os.WriteFile(output, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeGitHubOutputs(output, platformci.Plan{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`go_matrix={"include":[{"name":"not-selected","app_shard":""}]}`,
		`frontend_matrix={"include":[{"name":"not-selected"}]}`,
		"go_tests=false",
		"frontend_tests=false",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("GitHub output missing %q:\n%s", want, text)
		}
	}
}
