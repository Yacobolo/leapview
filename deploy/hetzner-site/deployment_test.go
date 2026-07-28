package hetznersite_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestComposeIsAStatelessTwoServiceSite(t *testing.T) {
	compose := readFile(t, filepath.Join("files", "compose.yaml"))
	var document struct {
		Services map[string]struct {
			Image       string            `yaml:"image"`
			Environment map[string]string `yaml:"environment"`
			Volumes     []string          `yaml:"volumes"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal([]byte(compose), &document); err != nil {
		t.Fatalf("parse Compose file: %v", err)
	}
	if len(document.Services) != 2 {
		t.Fatalf("services = %v, want exactly caddy and leapview-site", document.Services)
	}
	site, siteOK := document.Services["leapview-site"]
	caddy, caddyOK := document.Services["caddy"]
	if !siteOK || !caddyOK {
		t.Fatalf("services = %v, want exactly caddy and leapview-site", document.Services)
	}
	if len(site.Volumes) != 0 {
		t.Fatalf("leapview-site must not have application-state volumes: %v", site.Volumes)
	}
	if site.Environment["LEAPVIEW_SITE_BASE_URL"] != "https://leapview.dev" {
		t.Fatalf("LEAPVIEW_SITE_BASE_URL = %q", site.Environment["LEAPVIEW_SITE_BASE_URL"])
	}
	if _, configured := site.Environment["LEAPVIEW_SITE_SHOWCASE_EMBED_URL"]; configured {
		t.Fatal("the optional live-dashboard embed must remain unset for rc.1")
	}
	for name, contract := range map[string]struct {
		image string
		want  string
	}{
		"leapview-site": {image: site.Image, want: "${LEAPVIEW_SITE_IMAGE:?must be an immutable digest}"},
		"caddy":         {image: caddy.Image, want: "${CADDY_IMAGE:?must be an immutable digest}"},
	} {
		if contract.image != contract.want {
			t.Errorf("%s image = %q, want guarded immutable input %q", name, contract.image, contract.want)
		}
	}
	for _, forbidden := range []string{
		"duckdb", "leapview-state", "admin_email", "bootstrap", "backup",
		"ghcr.io/yacobolo/leapview@",
	} {
		if strings.Contains(strings.ToLower(compose), forbidden) {
			t.Errorf("site Compose contains forbidden product fragment %q", forbidden)
		}
	}
}

func TestTerraformProtectsThePermanentOrigin(t *testing.T) {
	main := readFile(t, "main.tf")
	outputs := readFile(t, "outputs.tf")
	for _, fragment := range []string{
		`resource "hcloud_primary_ip" "site"`,
		"auto_delete       = false",
		"delete_protection = true",
		`resource "hcloud_server" "site"`,
		"delete_protection        = true",
		"rebuild_protection       = true",
		"shutdown_before_deletion = true",
		"prevent_destroy = true",
		"ignore_changes = [user_data]",
		"backups                  = false",
		"ipv6_enabled = false",
	} {
		requireContains(t, main, fragment)
	}
	for _, fragment := range []string{
		`output "reserved_ipv4"`,
		`output "canonical_hostname"`,
		`output "deployment_target"`,
		`output "dns_records"`,
	} {
		requireContains(t, outputs, fragment)
	}
}

func TestTerraformRejectsMutableImagesAndWorldOpenSSH(t *testing.T) {
	variables := readFile(t, "variables.tf")
	for _, fragment := range []string{
		`variable "bootstrap_site_image"`,
		`variable "caddy_image"`,
		`@sha256:`,
		`variable "ssh_allowed_cidrs"`,
		`cidr != "0.0.0.0/0"`,
		`cidr != "::/0"`,
		`variable "operator_ssh_public_key"`,
	} {
		requireContains(t, variables, fragment)
	}
	if strings.Contains(variables, `default     = ["0.0.0.0/0", "::/0"]`) {
		t.Fatal("SSH must not be world-open by default")
	}
}

func TestBootstrapConsumesArtifactsWithoutBuildingOrInitializingProduct(t *testing.T) {
	main := readFile(t, "main.tf")
	cloudInit := readFile(t, "cloud-init.yaml.tftpl")
	provision := readFile(t, filepath.Join("files", "provision.sh"))
	for _, fragment := range []string{
		"compose_b64", "caddyfile_b64", "deployment_env_b64", "provision_b64",
	} {
		requireContains(t, main, fragment)
		requireContains(t, cloudInit, fragment)
	}
	for _, fragment := range []string{
		`docker pull "$LEAPVIEW_SITE_IMAGE"`,
		`docker pull "$CADDY_IMAGE"`,
		"config --quiet",
		"up --detach",
	} {
		requireContains(t, provision, fragment)
	}
	for _, forbidden := range []string{
		"git clone", "docker build", "leapviewctl init", "admin", "duckdb", "backup",
	} {
		if strings.Contains(strings.ToLower(cloudInit+"\n"+provision), forbidden) {
			t.Errorf("stateless bootstrap contains forbidden product operation %q", forbidden)
		}
	}
}

func TestRemoteStateAndReviewedApplyWorkflow(t *testing.T) {
	backend := readFile(t, "backend.tf")
	workflow := readFile(t, filepath.Join("..", "..", ".github", "workflows", "site-infrastructure.yml"))
	for _, fragment := range []string{
		`backend "s3"`,
		`key          = "leapview/site/production.tfstate"`,
		"use_lockfile = true",
	} {
		requireContains(t, backend, fragment)
	}
	for _, fragment := range []string{
		"name: Plan or apply permanent public-site infrastructure",
		"workflow_dispatch:",
		"group: leapview-site-infrastructure",
		"environment: leapview-site-production",
		"terraform plan",
		"terraform show -no-color",
		"aws s3api get-bucket-versioning",
		"actions/upload-artifact@",
		"retention-days: 90",
		"needs: plan",
		"terraform apply",
		"terraform plan -detailed-exitcode",
		"TF_STATE_BUCKET",
		"AWS_ENDPOINT_URL_S3",
		"TF_VAR_hcloud_token",
	} {
		requireContains(t, workflow, fragment)
	}
	for _, forbidden := range []string{
		"terraform destroy", "pull_request:", "-auto-approve",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("permanent infrastructure workflow contains forbidden fragment %q", forbidden)
		}
	}
}

func TestRepositoryCIValidatesPermanentSiteInfrastructure(t *testing.T) {
	taskfile := readFile(t, filepath.Join("..", "..", "Taskfile.yml"))
	ci := readFile(t, filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
	for _, fragment := range []string{
		"go test ./deploy/hetzner-site",
		"terraform fmt -check -recursive deploy/hetzner-site",
		"terraform -chdir=deploy/hetzner-site init -backend=false -input=false",
		"terraform -chdir=deploy/hetzner-site validate",
		"terraform -chdir=deploy/hetzner-site test",
	} {
		requireContains(t, taskfile, fragment)
	}
	for _, fragment := range []string{
		"go test ./deploy/compose ./deploy/hetzner ./deploy/hetzner-site",
		"deploy/hetzner-site/files/provision.sh",
		"working-directory: deploy/hetzner-site",
	} {
		requireContains(t, ci, fragment)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(contents)
}

func requireContains(t *testing.T, contents, fragment string) {
	t.Helper()
	if !strings.Contains(contents, fragment) {
		t.Fatalf("missing %q", fragment)
	}
}
