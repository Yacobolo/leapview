package maliciousinstance

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPackagedLeapViewPreservesRemoteContentBoundary(t *testing.T) {
	executable := os.Getenv("LEAPVIEW_PACKAGED_APP")
	if executable == "" {
		t.Skip("set LEAPVIEW_PACKAGED_APP to run the packaged LeapView proof")
	}
	externalServer := newExternalTargetServer(t)
	harness, err := New(Config{ExternalOrigin: externalServer.URL})
	if err != nil {
		t.Fatal(err)
	}
	server := newHarnessServer(t, harness)
	userData := t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), 35*time.Second)
	defer cancel()
	command := exec.CommandContext( //nolint:gosec // Explicit CI-built executable under test.
		ctx,
		executable,
		"--user-data-dir="+userData,
	)
	command.Env = append(
		os.Environ(),
		"LEAPVIEW_DESKTOP_PACKAGED_PROOF_ORIGIN="+server.URL,
	)
	output, runErr := command.CombinedOutput()
	payload, readErr := os.ReadFile(
		filepath.Join(userData, "packaged-security-proof.json"),
	)
	if readErr != nil {
		t.Fatalf(
			"read packaged proof result: %v\nprocess error: %v\noutput:\n%s",
			readErr,
			runErr,
			output,
		)
	}
	var result struct {
		SchemaVersion int    `json:"schemaVersion"`
		Passed        bool   `json:"passed"`
		Distribution  string `json:"distribution"`
	}
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("decode packaged proof result: %v\n%s", err, payload)
	}
	if runErr != nil || result.SchemaVersion != 1 || !result.Passed ||
		result.Distribution != "preview" {
		t.Fatalf(
			"packaged proof failed: process error=%v result=%s output=%s",
			runErr,
			payload,
			output,
		)
	}
}
