package compose

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"golang.org/x/mod/semver"
)

var (
	generateConfigOnce   sync.Once
	generateConfigOutput []byte
	generateConfigError  error
)

const configValidatorTestProgram = `package configvalidator

import (
	"testing"

	"github.com/Yacobolo/leapview/internal/app/config"
)

func TestProductionConfigValidator(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Production = true
	if err := cfg.Validate(config.ProfileServe); err != nil {
		t.Fatal(err)
	}
}
`

func TestComposeSingleInstanceContract(t *testing.T) {
	compose := read(t, "compose.yaml")
	for _, required := range []string{
		"leapview-state:/var/lib/leapview",
		"${COMPOSE_APP_BIND:-127.0.0.1:8080}:8080",
		"read_only: true",
		"cap_drop: [ALL]",
		"stop_grace_period: 2m",
		"type: tmpfs",
		"target: /tmp",
		"size: 536870912",
		"mode: 01777",
	} {
		if !strings.Contains(compose, required) {
			t.Fatalf("compose.yaml missing %q", required)
		}
	}
	if strings.Contains(compose, "container_name:") {
		t.Fatal("generic Compose must allow independent project names on one host")
	}
	if strings.Contains(compose, "./backups:/backups") {
		t.Fatal("backup archives must cross the container boundary as streams, not through a host bind with incompatible ownership")
	}
	if strings.Contains(compose, "/tmp:rw,noexec") {
		t.Fatal("tmpfs short syntax is rejected by Docker Desktop when its options are interpreted as mount paths")
	}
	appEnvironment := read(t, "leapview.env.example")
	for _, required := range []string{
		"LEAPVIEW_HOME=/var/lib/leapview/home",
		"LEAPVIEW_PUBLIC_URL=https://dash.example.com",
		"LEAPVIEW_ALLOWED_HOSTS=dash.example.com",
		"LEAPVIEW_TRUST_PROXY_HEADERS=true",
	} {
		if !strings.Contains(appEnvironment, required) {
			t.Fatalf("leapview.env.example missing %q", required)
		}
	}
	https := read(t, "compose.https.yaml")
	if !strings.Contains(https, "CADDY_IMAGE") || !strings.Contains(https, "443:443/udp") {
		t.Fatal("HTTPS overlay is incomplete")
	}
	for _, required := range []string{"type: tmpfs", "target: /tmp", "size: 67108864", "mode: 01777"} {
		if !strings.Contains(https, required) {
			t.Errorf("compose.https.yaml missing %q", required)
		}
	}
	if strings.Contains(https, "/tmp:rw,noexec") {
		t.Fatal("Caddy tmpfs short syntax is rejected by Docker Desktop when its options are interpreted as mount paths")
	}
}

func TestPublicImageIsPrimaryOnboardingContract(t *testing.T) {
	release := read(t, filepath.Join("..", "..", ".github", "workflows", "release.yml"))
	for _, required := range []string{
		"IMAGE_NAME: ghcr.io/yacobolo/leapview",
		"docker/setup-qemu-action@",
		`--tag "${IMAGE_NAME}:latest"`,
		"platforms: linux/amd64,linux/arm64",
		"Verify anonymous image pull",
		"docker logout ghcr.io",
		"docker buildx imagetools inspect",
	} {
		if !strings.Contains(release, required) {
			t.Fatalf("release workflow missing public image contract %q", required)
		}
	}
	if strings.Index(release, "docker/setup-qemu-action@") > strings.Index(release, "docker/setup-buildx-action@") {
		t.Fatal("release workflow must install emulation before creating the multi-platform builder")
	}

	for _, name := range []string{
		filepath.Join("..", "..", "README.md"),
		filepath.Join("..", "..", "docs", "articles", "start", "installation.md"),
	} {
		document := read(t, name)
		image := strings.Index(document, "ghcr.io/yacobolo/leapview:latest")
		pull := strings.Index(document, "docker pull")
		initialize := strings.Index(document, "admin initialize --format json")
		controller := strings.Index(document, "./leapviewctl init")
		if image < 0 || pull < 0 || initialize < 0 {
			t.Errorf("%s does not document pull-first public image onboarding", name)
		}
		if controller >= 0 && image > controller {
			t.Errorf("%s presents the operations controller before the public image", name)
		}
	}
}

func TestFiveMinuteEvaluationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	dockerfile := read(t, filepath.Join(root, "Dockerfile"))
	if !strings.Contains(dockerfile, "COPY evaluation ./evaluation") {
		t.Fatal("runtime image does not include the self-contained evaluation project and data")
	}
	dashboard := read(t, filepath.Join(root, "evaluation", "project", "workspaces", "evaluation", "dashboards", "sales-overview.yaml"))
	for _, required := range []string{
		"kind: static",
		"value: {kind: string, value: SP}",
		"value: {kind: string, value: RJ}",
		"value: {kind: string, value: MG}",
		"value: {kind: string, value: PR}",
	} {
		if !strings.Contains(dashboard, required) {
			t.Errorf("five-minute evaluation dashboard missing deterministic state option %q", required)
		}
	}
	for _, name := range []string{
		filepath.Join(root, "README.md"),
		filepath.Join(root, "docs", "articles", "start", "installation.md"),
	} {
		document := read(t, name)
		for _, required := range []string{
			"--name leapview-evaluate",
			"--publish 127.0.0.1:8080:8080",
			"--volume leapview-evaluate:/var/lib/leapview",
			"ghcr.io/yacobolo/leapview:latest evaluate",
			"docker exec leapview-evaluate leapview evaluate first-login",
			"docker rm --force leapview-evaluate",
			"docker volume rm leapview-evaluate",
			"Five-minute Sales Evaluation",
			"no source checkout",
		} {
			if !strings.Contains(document, required) {
				t.Errorf("%s missing five-minute evaluation contract %q", name, required)
			}
		}
	}
}

func TestInstalledCandidateQualificationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	release := read(t, filepath.Join(root, ".github", "workflows", "release.yml"))
	qualification := read(t, filepath.Join(root, ".github", "workflows", "installed-candidate.yml"))
	script := read(t, filepath.Join(root, "deploy", "compose", "qualification", "qualify.sh"))
	recovery := read(t, filepath.Join(root, "deploy", "compose", "qualification", "recover.sh"))
	browser := read(t, filepath.Join(root, "deploy", "compose", "qualification", "browser.mjs"))
	runbook := read(t, filepath.Join(root, "deploy", "compose", "QUALIFICATION.md"))

	for _, required := range []string{
		"cp -R deploy/compose/qualification",
		"./qualification/qualify.sh",
		"candidate-${{ github.run_id }}-${{ github.run_attempt }}",
		"docker buildx imagetools create",
		"gh release create",
		"needs: [image, qualify]",
	} {
		if !strings.Contains(release, required) {
			t.Errorf("release workflow missing installed-candidate gate %q", required)
		}
	}
	if strings.Contains(release, "types:\n      - published") {
		t.Fatal("release workflow cannot gate publication when it starts after a release is already public")
	}
	if strings.Index(release, "./qualification/qualify.sh") > strings.Index(release, "gh release create") {
		t.Fatal("release workflow publishes Compose archives before installed-candidate qualification")
	}
	if strings.Contains(release, "type=semver") {
		t.Fatal("release workflow must not publish versioned image tags before installed-candidate qualification")
	}
	if strings.Index(release, "./qualification/qualify.sh") > strings.Index(release, "docker buildx imagetools create") {
		t.Fatal("release workflow publishes versioned image tags before installed-candidate qualification")
	}

	for _, required := range []string{
		"workflow_dispatch:",
		"schedule:",
		"ubuntu-24.04-arm",
		"docker logout ghcr.io",
		"releases/download/",
		"sha256sum --check",
		"retention-days: 14",
		"Create qualification incident",
	} {
		if !strings.Contains(qualification, required) {
			t.Errorf("installed-candidate workflow missing %q", required)
		}
	}

	for _, required := range []string{
		"./leapviewctl init",
		"./leapviewctl start",
		"./leapviewctl first-login",
		`if [[ "$local_image" == true ]]; then`,
		`docker image inspect "$image_reference"`,
		"QUALIFICATION_MIN_FREE_BYTES",
		"evaluation/project/leapview.yaml",
		"./leapviewctl backup",
		"./leapviewctl restore",
		"docker restart",
		"/metrics",
		"auditedDenial",
		"runtime-identity.json",
		"qualification-report.json",
		"./qualification/recover.sh",
		"recovery-report.json",
	} {
		if !strings.Contains(script, required) {
			t.Errorf("qualification script missing tester assertion %q", required)
		}
	}
	for _, required := range []string{
		"managedUpload",
		"releaseFinalization",
		"deploymentActivation",
		"refreshRecovery",
		"queryStreamReconnect",
		"backupInterruption",
		"restorePreflight",
		"boundedDisk",
		"docker kill --signal KILL",
		"LEAPVIEW_REFRESH_JOB_LEASE_TIMEOUT",
		"listManagedDataUploadSessionEvents",
		"listDeploymentEvents",
		"listRefreshRunEvents",
		"exec ./leapviewctl backup interrupted.tar.gz",
		"exec ./leapviewctl restore backups/recovered.tar.gz",
	} {
		if !strings.Contains(recovery, required) {
			t.Errorf("recovery qualification missing fault assertion %q", required)
		}
	}
	waitForJSONStart := strings.Index(recovery, "wait_for_json() {")
	if waitForJSONStart < 0 {
		t.Fatal("recovery qualification is missing its JSON status waiter")
	}
	waitForJSONEnd := strings.Index(recovery[waitForJSONStart:], "\n}\n")
	if waitForJSONEnd < 0 {
		t.Fatal("recovery qualification JSON status waiter is unterminated")
	}
	waitForJSON := recovery[waitForJSONStart : waitForJSONStart+waitForJSONEnd]
	if !strings.Contains(waitForJSON, "sleep 1") {
		t.Error("recovery qualification must poll durable job status slowly enough to stay below the shipped API rate limit")
	}
	backupStart := strings.Index(recovery, `stage="backup interruption"`)
	if backupStart < 0 {
		t.Fatal("recovery qualification is missing the backup interruption stage")
	}
	backupStage := recovery[backupStart:]
	restoreStage := strings.Index(backupStage, `stage="restore preflight interruption"`)
	if restoreStage < 0 {
		t.Fatal("recovery qualification is missing the restore preflight stage")
	}
	backupStage = backupStage[:restoreStage]
	unthrottle := strings.Index(backupStage, `docker update --cpus 0 "$container_id"`)
	restart := strings.Index(backupStage, `./leapviewctl start`)
	if unthrottle < 0 || restart < 0 || unthrottle > restart {
		t.Error("backup recovery must remove its fault-injection CPU limit before restarting the service")
	}
	for _, workflow := range []struct {
		name     string
		contents string
	}{
		{name: "release", contents: release},
		{name: "installed candidate", contents: qualification},
	} {
		if !strings.Contains(workflow.contents, "recovery-report.json") {
			t.Errorf("%s workflow does not retain the bounded recovery report", workflow.name)
		}
	}
	if strings.Contains(script, "${run_suffix,,}") {
		t.Error("qualification script uses Bash 4 lowercase expansion and cannot run from the Darwin release bundle")
	}
	if !strings.Contains(script, `if [[ "$success" != true ]]; then`) {
		t.Error("qualification failure evidence must replace stale reports from a prior local run")
	}
	if !strings.Contains(script, `--output "$metrics_file"`) {
		t.Error("qualification must materialize metrics before searching them so pipefail cannot turn grep's early exit into a curl failure")
	}
	if !strings.Contains(script, `set_min_free_bytes "$restore_root"`) {
		t.Error("the local disk-reserve override must also apply to the isolated restore instance")
	}
	if !strings.Contains(script, `cp "$bundle_root/leapview.env" "$restore_root/leapview.env"`) {
		t.Error("isolated restore qualification must supply the original separately managed signing and encryption secrets")
	}
	if !strings.Contains(browser, `getByLabel('New password').press('Enter')`) {
		t.Error("browser qualification must submit the password form without relying on an animated button's stability")
	}
	for _, required := range []string{
		"page.goto(new URL(dashboardHref, baseURL)",
		"click({ force: true })",
		"locator('option', { hasText: 'SP' })",
		"selectOption({ label: 'SP' })",
		"/api/v1/workspaces/evaluation/groups",
		"authorization.denied",
		"MANAGE_GRANTS",
	} {
		if !strings.Contains(browser, required) {
			t.Errorf("browser qualification missing motion-independent interaction %q", required)
		}
	}

	for _, required := range []string{
		"Automated step",
		"Human check",
		"Five-minute Sales Evaluation",
		"Anonymous distribution",
		"Incident ownership",
		"Interruption recovery",
		"managed upload",
		"deployment activation",
		"refresh",
		"query/SSE",
		"backup",
		"restore preflight",
	} {
		if !strings.Contains(runbook, required) {
			t.Errorf("qualification runbook missing %q", required)
		}
	}
}

func TestControllerBuildAndLifecycleCommands(t *testing.T) {
	root := t.TempDir()
	controller := buildController(t, root)
	output, err := exec.Command(controller, "help").CombinedOutput()
	if err != nil {
		t.Fatalf("leapviewctl help: %v\n%s", err, output)
	}
	for _, command := range []string{"version", "init", "start", "status", "logs", "first-login", "backup", "restore", "upgrade", "rollback"} {
		if !strings.Contains(string(output), command) {
			t.Fatalf("controller help missing %s:\n%s", command, output)
		}
	}
}

func TestReleaseIdentityContract(t *testing.T) {
	root := filepath.Join("..", "..")
	version := strings.TrimSpace(read(t, filepath.Join(root, "VERSION")))
	if strings.HasPrefix(version, "v") || !semver.IsValid("v"+version) {
		t.Fatalf("VERSION = %q, want unprefixed semantic version", version)
	}
	packageManifest := read(t, filepath.Join(root, "package.json"))
	if !strings.Contains(packageManifest, `"version": "`+version+`"`) {
		t.Fatalf("package.json does not match VERSION %q", version)
	}

	dockerfile := read(t, filepath.Join(root, "Dockerfile"))
	for _, required := range []string{
		"BUILD_VERSION=development",
		"BUILD_REVISION=unknown",
		"BUILD_TIME=unknown",
		"BUILD_DIRTY=true",
		"BUILD_RELEASE=false",
		"internal/platform/buildinfo.version",
		"internal/platform/buildinfo.revision",
		"internal/platform/buildinfo.buildTime",
		"internal/platform/buildinfo.dirty",
		"internal/platform/buildinfo.release",
	} {
		if !strings.Contains(dockerfile, required) {
			t.Errorf("Dockerfile missing build identity contract %q", required)
		}
	}

	release := read(t, filepath.Join(root, ".github", "workflows", "release.yml"))
	for _, required := range []string{
		"Resolve authoritative build identity",
		"VERSION",
		"BUILD_TIME=",
		"BUILD_DIRTY=false",
		"BUILD_RELEASE=",
		"release-identity.json",
		"./leapviewctl version --json",
		"Verify published runtime identity",
		`docker run --rm "$IMAGE_REFERENCE" version --json`,
	} {
		if !strings.Contains(release, required) {
			t.Errorf("release workflow missing build identity contract %q", required)
		}
	}

	for _, name := range []string{
		filepath.Join(root, "deploy", "compose", "README.md"),
		filepath.Join(root, "docs", "articles", "start", "installation.md"),
	} {
		document := read(t, name)
		for _, required := range []string{
			"release-identity.json",
			"leapviewctl version --json",
			"org.opencontainers.image.version",
			"/api/v1/capabilities",
		} {
			if !strings.Contains(document, required) {
				t.Errorf("%s missing identity verification step %q", name, required)
			}
		}
	}
}

func TestControllerInitializationGeneratesValidPublicOrigin(t *testing.T) {
	binaryDir := t.TempDir()
	validator := buildConfigValidator(t, binaryDir)
	image := "example.com/leapview@sha256:" + strings.Repeat("a", 64)

	for _, test := range []struct {
		name       string
		args       []string
		composeTLS string
	}{
		{name: "built-in Caddy", composeTLS: "1"},
		{name: "trusted external HTTPS proxy", args: []string{"--no-https"}, composeTLS: "0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			buildController(t, root)
			copyDeploymentFile(t, root, "deployment.env.example", 0o600)
			fakeDocker := filepath.Join(root, "fake-docker")
			script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
root="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
if [[ " $* " == *" config validate --production "* ]]; then
  set -a
  source "$root/leapview.env"
  set +a
  exec %q config validate --production
fi
if [[ " $* " == *" admin initialize --format json "* ]]; then
  printf '{"email":"admin@example.com","temporaryPassword":"temporary","publisherToken":"publisher","publisherTokenExpiresAt":"2026-07-19T00:00:00Z"}\n'
fi
`, validator)
			if err := os.WriteFile(fakeDocker, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}

			args := []string{"init", "--admin-email", "admin@example.com", "--domain", "dash.example.com", "--image", image}
			args = append(args, test.args...)
			runController(t, root, fakeDocker, "", args...)

			appEnvironment := readFile(t, filepath.Join(root, "leapview.env"))
			for _, required := range []string{
				"LEAPVIEW_PUBLIC_URL=https://dash.example.com\n",
				"LEAPVIEW_ALLOWED_HOSTS=dash.example.com\n",
				"LEAPVIEW_TRUST_PROXY_HEADERS=true\n",
			} {
				if !strings.Contains(appEnvironment, required) {
					t.Errorf("leapview.env missing %q:\n%s", required, appEnvironment)
				}
			}
			deploymentEnvironment := readFile(t, filepath.Join(root, "deployment.env"))
			for _, required := range []string{
				"CADDY_DOMAIN=dash.example.com\n",
				"COMPOSE_HTTPS=" + test.composeTLS + "\n",
			} {
				if !strings.Contains(deploymentEnvironment, required) {
					t.Errorf("deployment.env missing %q:\n%s", required, deploymentEnvironment)
				}
			}
		})
	}
}

func TestControllerReleasePackagingContract(t *testing.T) {
	release := read(t, filepath.Join("..", "..", ".github", "workflows", "release.yml"))
	for _, required := range []string{
		"./cmd/leapviewctl",
		"CGO_ENABLED=0",
		"linux amd64",
		"linux arm64",
		"darwin amd64",
		"darwin arm64",
	} {
		if !strings.Contains(release, required) {
			t.Fatalf("release workflow missing Go controller packaging contract %q", required)
		}
	}
	dockerfile := read(t, filepath.Join("..", "..", "Dockerfile"))
	if !strings.Contains(dockerfile, "/usr/local/libexec/leapviewctl") {
		t.Fatal("application image must carry the matching Linux controller for provider extraction")
	}
}

func TestControllerLifecycleWithStateAwareUpgradeRollback(t *testing.T) {
	root := t.TempDir()
	buildController(t, root)
	buildConfigValidator(t, root)
	copyDeploymentFile(t, root, "deployment.env.example", 0o600)
	fakeDocker := filepath.Join(root, "fake-docker")
	if err := os.WriteFile(fakeDocker, []byte(`#!/usr/bin/env bash
set -euo pipefail
root="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
printf '%s\n' "$*" >>"$root/docker.log"
if [[ -n "${FAKE_DOCKER_FAIL_COMMAND:-}" && " $* " == *" ${FAKE_DOCKER_FAIL_COMMAND} "* ]]; then exit 42; fi
if [[ "${FAKE_DOCKER_FAIL_RESTORE_ONCE:-}" == 1 && " $* " == *' admin restore '* && ! -e "$root/restore-failed-once" ]]; then
  touch "$root/restore-failed-once"
  exit 42
fi
if [[ "${1:-}" == inspect ]]; then
  template="${3:-}"
  if [[ "$template" == *Running* ]]; then printf 'true\n'; exit 0; fi
  image="$(awk -F= '$1=="LEAPVIEW_IMAGE" {sub(/^[^=]*=/, ""); print; exit}' "$root/deployment.env")"
  if [[ -n "${FAKE_DOCKER_FAIL_IMAGE:-}" && "$image" == "$FAKE_DOCKER_FAIL_IMAGE" ]]; then printf 'unhealthy\n'; else printf 'healthy\n'; fi
  exit 0
fi
[[ "${1:-}" == compose ]] || exit 0
shift
while [[ $# -gt 0 ]]; do
  case "$1" in
    --project-directory|--env-file|-f) shift 2 ;;
    *) command="$1"; shift; break ;;
  esac
done
case "${command:-}" in
  ps) [[ " $* " == *' -q '* ]] && printf 'fake-container\n' ;;
  run)
    if [[ " $* " == *' config validate --production '* ]]; then
      set -a
      source "$root/leapview.env"
      set +a
      exec "$root/config-validator"
    elif [[ " $* " == *' admin initialize --format json '* ]]; then
      printf '{"email":"admin@example.com","temporaryPassword":"temporary","publisherToken":"publisher","publisherTokenExpiresAt":"2026-07-19T00:00:00Z"}\n'
    elif [[ " $* " == *' admin backup '* ]]; then
      output=""
      while [[ $# -gt 0 ]]; do
        if [[ "$1" == --out ]]; then output="$2"; break; fi
        shift
      done
      if [[ "$output" == - ]]; then
        printf 'validated archive\n'
      else
        output="$root/${output#/}"
        mkdir -p "$(dirname -- "$output")"
        printf 'validated archive\n' >"$output"
      fi
    fi
    ;;
esac
`), 0o700); err != nil {
		t.Fatal(err)
	}

	oldImage := "example.com/leapview@sha256:" + strings.Repeat("a", 64)
	newImage := "example.com/leapview@sha256:" + strings.Repeat("b", 64)
	runController(t, root, fakeDocker, "", "init", "--admin-email", "admin@example.com", "--domain", "dash.example.com", "--image", oldImage)
	for _, name := range []string{"deployment.env", "leapview.env", "initial-credentials.json"} {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("%s permissions = %v, %v", name, info.Mode().Perm(), err)
		}
	}
	if output := runController(t, root, fakeDocker, "", "first-login"); !strings.Contains(output, `"temporaryPassword":"temporary"`) {
		t.Fatalf("first-login output = %s", output)
	}
	if _, err := os.Stat(filepath.Join(root, "initial-credentials.json")); !os.IsNotExist(err) {
		t.Fatalf("one-time credentials were not deleted: %v", err)
	}
	runController(t, root, fakeDocker, "", "start")
	t.Setenv("FAKE_DOCKER_FAIL_COMMAND", "admin backup")
	if output, err := runControllerResult(root, fakeDocker, "", "backup"); err == nil || !strings.Contains(output, "previous service state was restored") {
		t.Fatalf("failed backup result = %v, %s", err, output)
	}
	t.Setenv("FAKE_DOCKER_FAIL_COMMAND", "")
	backupOutput := runController(t, root, fakeDocker, "", "backup")
	backupPath := strings.TrimSpace(backupOutput)
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup missing: %v (%s)", err, backupOutput)
	}
	runController(t, root, fakeDocker, "", "restore", backupPath)
	t.Setenv("FAKE_DOCKER_FAIL_COMMAND", "pull leapview")
	if output, err := runControllerResult(root, fakeDocker, "", "upgrade", newImage); err == nil || !strings.Contains(output, "previous image and service state were restored") {
		t.Fatalf("failed pull result = %v, %s", err, output)
	}
	requireDeploymentImage(t, root, oldImage)
	t.Setenv("FAKE_DOCKER_FAIL_COMMAND", "")

	output, err := runControllerResult(root, fakeDocker, newImage, "upgrade", newImage)
	if err == nil || !strings.Contains(output, "previous image and state were restored") {
		t.Fatalf("failed upgrade result = %v, %s", err, output)
	}
	requireDeploymentImage(t, root, oldImage)
	runController(t, root, fakeDocker, "", "upgrade", newImage)
	requireDeploymentImage(t, root, newImage)
	t.Setenv("FAKE_DOCKER_FAIL_RESTORE_ONCE", "1")
	if output, err := runControllerResult(root, fakeDocker, "", "rollback", "--confirm"); err == nil || !strings.Contains(output, "pre-rollback image and state were reinstated") {
		t.Fatalf("failed rollback result = %v, %s", err, output)
	}
	requireDeploymentImage(t, root, newImage)
	t.Setenv("FAKE_DOCKER_FAIL_RESTORE_ONCE", "")
	runController(t, root, fakeDocker, "", "rollback", "--confirm")
	requireDeploymentImage(t, root, oldImage)
	log, err := os.ReadFile(filepath.Join(root, "docker.log"))
	if err != nil || !strings.Contains(string(log), "admin restore") {
		t.Fatalf("controller did not restore paired state: %v\n%s", err, log)
	}
}

func TestControllerInitializationIsRetryableAndRequiresPinnedProxy(t *testing.T) {
	image := "example.com/leapview@sha256:" + strings.Repeat("a", 64)
	setup := func(t *testing.T) (string, string) {
		t.Helper()
		root := t.TempDir()
		buildController(t, root)
		buildConfigValidator(t, root)
		copyDeploymentFile(t, root, "deployment.env.example", 0o600)
		fakeDocker := filepath.Join(root, "fake-docker")
		if err := os.WriteFile(fakeDocker, []byte(`#!/usr/bin/env bash
set -euo pipefail
root="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
if [[ -f "$root/fail-validation" && " $* " == *" config validate "* ]]; then exit 42; fi
if [[ " $* " == *" config validate --production "* ]]; then
  set -a
  source "$root/leapview.env"
  set +a
  exec "$root/config-validator"
elif [[ " $* " == *" admin initialize --format json "* ]]; then
  printf '{"email":"admin@example.com","temporaryPassword":"temporary","publisherToken":"publisher","publisherTokenExpiresAt":"2026-07-19T00:00:00Z"}\n'
fi
`), 0o700); err != nil {
			t.Fatal(err)
		}
		return root, fakeDocker
	}

	t.Run("retry after validation failure", func(t *testing.T) {
		root, fakeDocker := setup(t)
		if err := os.WriteFile(filepath.Join(root, "fail-validation"), []byte("fail\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if output, err := runControllerResult(root, fakeDocker, "", "init", "--admin-email", "admin@example.com", "--domain", "dash.example.com", "--image", image); err == nil || !strings.Contains(output, "initialization can be retried") {
			t.Fatalf("failed initialization = %v, %s", err, output)
		}
		for _, name := range []string{"leapview.env", "initial-credentials.json"} {
			if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
				t.Fatalf("partial initialization retained %s: %v", name, err)
			}
		}
		if err := os.Remove(filepath.Join(root, "fail-validation")); err != nil {
			t.Fatal(err)
		}
		runController(t, root, fakeDocker, "", "init", "--admin-email", "admin@example.com", "--domain", "dash.example.com", "--image", image)
	})

	t.Run("mutable proxy image", func(t *testing.T) {
		root, fakeDocker := setup(t)
		examplePath := filepath.Join(root, "deployment.env.example")
		contents, err := os.ReadFile(examplePath)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(contents), "\n")
		for i := range lines {
			if strings.HasPrefix(lines[i], "CADDY_IMAGE=") {
				lines[i] = "CADDY_IMAGE=caddy:latest"
			}
		}
		if err := os.WriteFile(examplePath, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
			t.Fatal(err)
		}
		if output, err := runControllerResult(root, fakeDocker, "", "init", "--admin-email", "admin@example.com", "--domain", "dash.example.com", "--image", image); err == nil || !strings.Contains(output, "image must be pinned by digest") {
			t.Fatalf("mutable proxy result = %v, %s", err, output)
		}
	})
}

func copyDeploymentFile(t *testing.T, targetDir, name string, mode os.FileMode) {
	t.Helper()
	contents, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, name), contents, mode); err != nil {
		t.Fatal(err)
	}
}

func buildController(t *testing.T, targetDir string) string {
	t.Helper()
	target := filepath.Join(targetDir, "leapviewctl")
	command := exec.Command("go", "build", "-o", target, "./cmd/leapviewctl")
	command.Dir = filepath.Join("..", "..")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build leapviewctl: %v\n%s", err, output)
	}
	return target
}

func buildConfigValidator(t *testing.T, targetDir string) string {
	t.Helper()
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	generateConfigOnce.Do(func() {
		command := exec.Command("go", "run", "./internal/app/tools/configgen")
		command.Dir = repositoryRoot
		generateConfigOutput, generateConfigError = command.CombinedOutput()
	})
	if generateConfigError != nil {
		t.Fatalf("generate configuration contract: %v\n%s", generateConfigError, generateConfigOutput)
	}

	temporaryRoot := filepath.Join(repositoryRoot, ".tmp")
	if err := os.MkdirAll(temporaryRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	sourceDir, err := os.MkdirTemp(temporaryRoot, "compose-config-validator-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(sourceDir) })
	if err := os.WriteFile(filepath.Join(sourceDir, "validator_test.go"), []byte(configValidatorTestProgram), 0o600); err != nil {
		t.Fatal(err)
	}
	relativeSourceDir, err := filepath.Rel(repositoryRoot, sourceDir)
	if err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(targetDir, "config-validator")
	command := exec.Command("go", "test", "-c", "-o", target, "./"+filepath.ToSlash(relativeSourceDir))
	command.Dir = repositoryRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build production config validator: %v\n%s", err, output)
	}
	return target
}

func runController(t *testing.T, root, docker, failImage string, args ...string) string {
	t.Helper()
	output, err := runControllerResult(root, docker, failImage, args...)
	if err != nil {
		t.Fatalf("leapviewctl %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return output
}

func runControllerResult(root, docker, failImage string, args ...string) (string, error) {
	command := exec.Command(filepath.Join(root, "leapviewctl"), args...)
	command.Dir = root
	command.Env = append(os.Environ(), "LEAPVIEWCTL_ROOT="+root, "LEAPVIEWCTL_DOCKER_BIN="+docker, "FAKE_DOCKER_FAIL_IMAGE="+failImage)
	output, err := command.CombinedOutput()
	return string(output), err
}

func requireDeploymentImage(t *testing.T, root, image string) {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(root, "deployment.env"))
	if err != nil || !strings.Contains(string(contents), "LEAPVIEW_IMAGE="+image+"\n") {
		t.Fatalf("deployment image is not %s: %v\n%s", image, err, contents)
	}
}

func read(t *testing.T, name string) string {
	t.Helper()
	return readFile(t, name)
}

func readFile(t *testing.T, name string) string {
	t.Helper()
	value, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(value)
}
