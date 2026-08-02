package githubaction_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSetupActionHasSecureProjectAwareContract(t *testing.T) {
	data := read(t, filepath.Join("..", "..", "action", "setup-rtest", "action.yml"))
	for _, want := range []string{
		"using: composite",
		"version:",
		"repository:",
		"allow-source-fallback:",
		"service-url:",
		"project:",
		"image:",
		"ca-certificate:",
		"oidc-audience:",
		"backend: \"service\"",
		"actions/cache/restore@",
		"actions/cache/save@",
		"install-release.sh",
		"chmod 0600",
		"RTEST_CONFIG",
		"GITHUB_PATH",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("action.yml missing %q", want)
		}
	}
	if strings.Contains(data, "go build -trimpath -o \"${bin_dir}/rtest\"") {
		t.Fatal("action.yml must not unconditionally compile rtest")
	}
}

func TestReleaseInstallerDownloadsAndVerifiesPinnedArchive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release installer targets POSIX GitHub runners")
	}
	releaseRoot := t.TempDir()
	installRoot := t.TempDir()
	version := "9.8.7"
	asset := "rtest_" + version + "_" + installerOS(runtime.GOOS) + "_" + installerArch(runtime.GOARCH) + ".tar.gz"
	assetDir := filepath.Join(releaseRoot, "rtest-v"+version)
	if err := os.MkdirAll(filepath.Join(assetDir, "payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(assetDir, "payload", "rtest")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf '%s\\n' '"+version+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, assetDir, "tar", "-czf", asset, "-C", "payload", "rtest")
	archive, err := os.ReadFile(filepath.Join(assetDir, asset))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(archive)
	checksums := hex.EncodeToString(digest[:]) + "  " + asset + "\n"
	if err := os.WriteFile(filepath.Join(assetDir, "checksums.txt"), []byte(checksums), 0o644); err != nil {
		t.Fatal(err)
	}

	result := runInstaller(t, map[string]string{
		"RTEST_VERSION":          version,
		"RTEST_REPOSITORY":       "example/rtest",
		"RTEST_RELEASE_BASE_URL": "file://" + releaseRoot,
		"RTEST_INSTALL_ROOT":     installRoot,
	})
	if !strings.Contains(result, "source=release") {
		t.Fatalf("installer output = %q, want release source", result)
	}
	got := run(t, t.TempDir(), filepath.Join(installRoot, "bin", "rtest"), "version")
	if strings.TrimSpace(got) != version {
		t.Fatalf("installed version = %q, want %q", strings.TrimSpace(got), version)
	}
	cached := runInstaller(t, map[string]string{
		"RTEST_VERSION":          version,
		"RTEST_REPOSITORY":       "example/rtest",
		"RTEST_RELEASE_BASE_URL": "file://" + t.TempDir(),
		"RTEST_INSTALL_ROOT":     installRoot,
	})
	if !strings.Contains(cached, "source=cache") {
		t.Fatalf("second installer output = %q, want cache source", cached)
	}
}

func TestReleaseInstallerRejectsChecksumMismatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release installer targets POSIX GitHub runners")
	}
	releaseRoot := t.TempDir()
	version := "9.8.7"
	asset := "rtest_" + version + "_" + installerOS(runtime.GOOS) + "_" + installerArch(runtime.GOARCH) + ".tar.gz"
	assetDir := filepath.Join(releaseRoot, "rtest-v"+version)
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, asset), []byte("corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "checksums.txt"), []byte(strings.Repeat("0", 64)+"  "+asset+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	output, err := installerCommand(map[string]string{
		"RTEST_VERSION":          version,
		"RTEST_REPOSITORY":       "example/rtest",
		"RTEST_RELEASE_BASE_URL": "file://" + releaseRoot,
		"RTEST_INSTALL_ROOT":     t.TempDir(),
	}).CombinedOutput()
	if err == nil {
		t.Fatalf("installer unexpectedly accepted mismatched checksum: %s", output)
	}
	if !strings.Contains(string(output), "checksum") {
		t.Fatalf("installer error %q does not identify checksum failure", output)
	}
}

func TestReleaseInstallerUsesExplicitSourceFallbackWithoutRelease(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release installer targets POSIX GitHub runners")
	}
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "cmd", "rtest"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "go.mod"), []byte("module example.test/rtest\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	program := "package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"9.8.7\") }\n"
	if err := os.WriteFile(filepath.Join(source, "cmd", "rtest", "main.go"), []byte(program), 0o644); err != nil {
		t.Fatal(err)
	}
	result := runInstaller(t, map[string]string{
		"RTEST_VERSION":               "9.8.7",
		"RTEST_REPOSITORY":            "example/rtest",
		"RTEST_RELEASE_BASE_URL":      "file://" + t.TempDir(),
		"RTEST_INSTALL_ROOT":          t.TempDir(),
		"RTEST_ACTION_ROOT":           source,
		"RTEST_ALLOW_SOURCE_FALLBACK": "true",
	})
	if !strings.Contains(result, "source=source") {
		t.Fatalf("installer output = %q, want source fallback", result)
	}
}

func TestReleaseInstallerSelectsGitHubRunnerPlatform(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release installer targets POSIX GitHub runners")
	}
	releaseRoot := t.TempDir()
	version := "9.8.7"
	asset := "rtest_" + version + "_linux_arm64.tar.gz"
	assetDir := filepath.Join(releaseRoot, "rtest-v"+version)
	if err := os.MkdirAll(filepath.Join(assetDir, "payload"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "payload", "rtest"), []byte("#!/bin/sh\necho "+version+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, assetDir, "tar", "-czf", asset, "-C", "payload", "rtest")
	archive, err := os.ReadFile(filepath.Join(assetDir, asset))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(archive)
	if err := os.WriteFile(filepath.Join(assetDir, "checksums.txt"), []byte(hex.EncodeToString(digest[:])+"  "+asset+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result := runInstaller(t, map[string]string{
		"RUNNER_OS":              "Linux",
		"RUNNER_ARCH":            "ARM64",
		"RTEST_VERSION":          version,
		"RTEST_REPOSITORY":       "example/rtest",
		"RTEST_RELEASE_BASE_URL": "file://" + releaseRoot,
		"RTEST_INSTALL_ROOT":     t.TempDir(),
	})
	if !strings.Contains(result, "source=release") {
		t.Fatalf("installer output = %q, want linux/arm64 release", result)
	}
}

func TestReleaseWorkflowPackagesPortableChecksummedAssets(t *testing.T) {
	data := read(t, filepath.Join("..", "..", "..", ".github", "workflows", "rtest-release.yml"))
	for _, want := range []string{
		"rtest-v*",
		"linux amd64",
		"linux arm64",
		"darwin amd64",
		"darwin arm64",
		"checksums.txt",
		"gh release create",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("rtest-release.yml missing %q", want)
		}
	}
}

func TestPOCWorkflowIsManualOIDCAndRepositoryScoped(t *testing.T) {
	data := read(t, filepath.Join("..", "..", "..", ".github", "workflows", "rtest-poc.yml"))
	for _, want := range []string{
		"workflow_dispatch:",
		"id-token: write",
		"github.repository == 'flidai/leapview'",
		"environment: rtest-poc",
		"uses: ./rtest/action/setup-rtest",
		"service-url: ${{ vars.RTEST_SERVICE_URL }}",
		"project: poc",
		"rtest doctor",
		"rtest exec -- go test -count=1 -v ./...",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("rtest-poc.yml missing %q", want)
		}
	}
	if strings.Contains(data, "pull_request:") {
		t.Fatal("POC workflow must not run automatically for pull requests")
	}
	for _, forbidden := range []string{"tailscale/github-action", "ssh-private-key", "RTEST_TOKEN"} {
		if strings.Contains(data, forbidden) {
			t.Fatalf("POC workflow still contains legacy authentication %q", forbidden)
		}
	}
}

func TestMainCIUsesOIDCForTrustedProjectBuilds(t *testing.T) {
	data := read(t, filepath.Join("..", "..", "..", ".github", "workflows", "ci.yml"))
	for _, want := range []string{
		"site-image:",
		"production-image:",
		"github.event.pull_request.head.repo.full_name == github.repository",
		"github.actor != 'dependabot[bot]'",
		"environment: rtest-poc",
		"id-token: write",
		"uses: ./rtest/action/setup-rtest",
		"project: leapview",
		"rtest build --",
	} {
		if !strings.Contains(data, want) {
			t.Fatalf("ci.yml missing trusted rtest build gate %q", want)
		}
	}
	if strings.Contains(data, "rtest-oidc-e2e:") {
		t.Fatal("main CI must not retain the redundant POC-only OIDC job")
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
