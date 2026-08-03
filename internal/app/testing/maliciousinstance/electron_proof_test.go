package maliciousinstance

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestElectronPolicyIntegrationPreservesBrowserEquivalentAuthority(t *testing.T) {
	switch runtime.GOOS {
	case "darwin", "linux", "windows":
	default:
		t.Skipf("Electron policy integration is unsupported on %s", runtime.GOOS)
	}
	electronBinary := os.Getenv("LEAPVIEW_ELECTRON_BINARY")
	if electronBinary == "" {
		t.Skip("set LEAPVIEW_ELECTRON_BINARY to run the pinned Electron policy integration")
	}

	externalServer := newExternalTargetServer(t)
	harness, err := New(Config{ExternalOrigin: externalServer.URL})
	if err != nil {
		t.Fatal(err)
	}
	server := newHarnessServer(t, harness)

	resultPath := filepath.Join(t.TempDir(), "electron-proof.json")
	runnerPath := filepath.Join("electron", "runner.mjs")
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, electronBinary, runnerPath) //nolint:gosec // The explicit test-only binary is provided by the caller.
	command.Env = append(os.Environ(),
		"LEAPVIEW_PROOF_ORIGIN="+server.URL,
		"LEAPVIEW_PROOF_RESULT="+resultPath,
	)
	output, runErr := command.CombinedOutput()

	payload, readErr := os.ReadFile(resultPath)
	if readErr != nil {
		t.Fatalf("read Electron proof result: %v\nprocess error: %v\noutput:\n%s", readErr, runErr, output)
	}
	var result struct {
		Passed          bool          `json:"passed"`
		Framework       string        `json:"framework"`
		Chromium        string        `json:"chromium"`
		ManifestVersion string        `json:"manifestVersion"`
		Observations    []Observation `json:"observations"`
		Error           string        `json:"error"`
	}
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("decode Electron proof result: %v\n%s", err, payload)
	}
	if runErr != nil || !result.Passed {
		t.Fatalf("Electron proof failed: process error=%v result=%s output=%s", runErr, payload, output)
	}
	manifest := harness.Manifest()
	if result.ManifestVersion != manifest.Version {
		t.Fatalf("Electron proof manifest version = %q, want %q", result.ManifestVersion, manifest.Version)
	}
	if len(result.Observations) != len(manifest.Attacks) {
		t.Fatalf("Electron proof observations = %v, want exactly %d", result.Observations, len(manifest.Attacks))
	}
	seen := make(map[string]Outcome, len(result.Observations))
	for _, observation := range result.Observations {
		if _, duplicate := seen[observation.AttackID]; duplicate {
			t.Fatalf("Electron proof contains duplicate observation %q", observation.AttackID)
		}
		seen[observation.AttackID] = observation.Outcome
	}
	for _, attack := range manifest.Attacks {
		if got, ok := seen[attack.ID]; !ok || got != attack.Expected {
			t.Fatalf("Electron proof observation %q = %q, present=%v, want %q", attack.ID, got, ok, attack.Expected)
		}
	}
	t.Logf("%s / Chromium %s satisfied all %d security invariants", result.Framework, result.Chromium, len(result.Observations))
}
