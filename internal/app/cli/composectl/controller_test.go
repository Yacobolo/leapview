package composectl

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	instancelock "github.com/flidai/leapview/internal/platform/locking"
)

func TestControllerLockRejectsConcurrentOperationAndRecoversAfterRelease(t *testing.T) {
	root := t.TempDir()
	first, err := instancelock.AcquireNamed(root, controllerLockName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := instancelock.AcquireNamed(root, controllerLockName); err == nil || !strings.Contains(err.Error(), "another process") {
		t.Fatalf("concurrent lock error = %v", err)
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	second, err := instancelock.AcquireNamed(root, controllerLockName)
	if err != nil {
		t.Fatalf("reacquire released lock: %v", err)
	}
	defer second.Release()
}

func TestRemoveInterruptedBackupArchivesPreservesCompletedBackups(t *testing.T) {
	directory := t.TempDir()
	stale := filepath.Join(directory, ".leapview-backup-interrupted.tmp")
	completed := filepath.Join(directory, "completed.tar.gz")
	if err := os.WriteFile(stale, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(completed, []byte("complete"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := removeInterruptedBackupArchives(directory); err != nil {
		t.Fatalf("removeInterruptedBackupArchives() error = %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("interrupted backup survived: %v", err)
	}
	if contents, err := os.ReadFile(completed); err != nil || string(contents) != "complete" {
		t.Fatalf("completed backup = %q, %v", contents, err)
	}
}

func TestUpgradeRejectsReleasedV010BeforeDockerOrStateMutation(t *testing.T) {
	root := t.TempDir()
	const releasedV010 = "ghcr.io/yacobolo/libredash@sha256:677caaf256cb3a0d61efd47b289debbd91984976a5a5c4b372196a5d79ce7153"
	next := "ghcr.io/flidai/leapview@sha256:" + strings.Repeat("a", 64)
	deployment := "LEAPVIEW_IMAGE=" + releasedV010 + "\nCOMPOSE_HTTPS=0\n"
	if err := os.WriteFile(filepath.Join(root, deploymentEnvName), []byte(deployment), 0o600); err != nil {
		t.Fatal(err)
	}
	controller, err := New(Options{Root: root, DockerBin: "/bin/false"})
	if err != nil {
		t.Fatal(err)
	}

	err = controller.Upgrade(t.Context(), next)
	if err == nil || !strings.Contains(err.Error(), "v0.1.0") || !strings.Contains(err.Error(), "fresh-install-only") {
		t.Fatalf("Upgrade() error = %v, want explicit v0.1.0 incompatibility", err)
	}
	if contents, err := os.ReadFile(filepath.Join(root, deploymentEnvName)); err != nil || string(contents) != deployment {
		t.Fatalf("deployment state changed before rejection: %q, %v", contents, err)
	}
	for _, path := range []string{rollbackEnvName, "backups"} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("upgrade rejection created %s: %v", path, err)
		}
	}
}

func TestFirstLoginRetainsCredentialsUntilOutputSucceeds(t *testing.T) {
	root := t.TempDir()
	credentialsPath := filepath.Join(root, credentialsName)
	credentials := []byte("{\"temporaryPassword\":\"temporary\"}\n")
	if err := os.WriteFile(credentialsPath, credentials, 0o600); err != nil {
		t.Fatal(err)
	}
	controller, err := New(Options{Root: root, Stdout: failingWriter{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := controller.FirstLogin(); err == nil {
		t.Fatal("first-login output failure = nil")
	}
	if contents, err := os.ReadFile(credentialsPath); err != nil || !bytes.Equal(contents, credentials) {
		t.Fatalf("credentials after output failure = %q, %v", contents, err)
	}

	var output bytes.Buffer
	controller, err = New(Options{Root: root, Stdout: &output})
	if err != nil {
		t.Fatal(err)
	}
	if err := controller.FirstLogin(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output.Bytes(), credentials) {
		t.Fatalf("first-login output = %q", output.Bytes())
	}
	if _, err := os.Stat(credentialsPath); !os.IsNotExist(err) {
		t.Fatalf("credentials remain after successful output: %v", err)
	}
}

func TestUpdateEnvFileIsPrivateAndRejectsMissingContractKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deployment.env")
	if err := os.WriteFile(path, []byte("LEAPVIEW_IMAGE=old\nCOMPOSE_HTTPS=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := updateEnvFile(path, map[string]string{"LEAPVIEW_IMAGE": "new"}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "LEAPVIEW_IMAGE=new\nCOMPOSE_HTTPS=1\n" {
		t.Fatalf("updated environment = %q, %v", contents, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("environment permissions = %v", info.Mode().Perm())
	}
	if err := updateEnvFile(path, map[string]string{"CADDY_DOMAIN": "dash.example.com"}); err == nil {
		t.Fatal("missing environment key update succeeded")
	}
}

func TestEnvironmentLineValuesRejectConfigurationInjection(t *testing.T) {
	for _, value := range []string{"prod\nLEAPVIEW_CSRF_KEY=forged", "dash.example.com\rCOMPOSE_HTTPS=0", "admin@example.com\x00suffix"} {
		if err := validateEnvLineValue("test value", value); err == nil {
			t.Fatalf("configuration injection value %q was accepted", value)
		}
	}
	if err := validateEnvLineValue("domain", "dash.example.com"); err != nil {
		t.Fatalf("ordinary value rejected: %v", err)
	}
}

func TestInitializeRejectsInvalidPublicDomainBeforeStateMutation(t *testing.T) {
	root := t.TempDir()
	example := "LEAPVIEW_IMAGE=example.com/leapview@sha256:" + strings.Repeat("a", 64) +
		"\nCADDY_IMAGE=example.com/caddy@sha256:" + strings.Repeat("b", 64) +
		"\nCADDY_DOMAIN=dash.example.com\nCOMPOSE_HTTPS=1\n"
	if err := os.WriteFile(filepath.Join(root, "deployment.env.example"), []byte(example), 0o600); err != nil {
		t.Fatal(err)
	}
	controller, err := New(Options{Root: root, DockerBin: "/bin/false"})
	if err != nil {
		t.Fatal(err)
	}

	err = controller.Initialize(context.Background(), InitOptions{
		AdminEmail: "admin@example.com",
		Domain:     "https://dash.example.com",
	})
	if err == nil || !strings.Contains(err.Error(), "--domain must be a hostname") {
		t.Fatalf("invalid public domain error = %v", err)
	}
	for _, name := range []string{deploymentEnvName, appEnvName, credentialsName, controllerLockName} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Errorf("invalid public domain mutated %s: %v", name, err)
		}
	}
}

func TestCanonicalPublicDomain(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{input: "dash.example.com", want: "dash.example.com"},
		{input: " Dash.Example.COM. ", want: "dash.example.com"},
		{input: "localhost", want: "localhost"},
	} {
		t.Run(test.input, func(t *testing.T) {
			got, err := canonicalPublicDomain(test.input)
			if err != nil || got != test.want {
				t.Fatalf("canonicalPublicDomain(%q) = %q, %v; want %q", test.input, got, err, test.want)
			}
		})
	}
	for _, input := range []string{
		"https://dash.example.com",
		"dash.example.com/path",
		"dash.example.com:8443",
		"user@dash.example.com",
		"*.example.com",
		"-dash.example.com",
		"dash..example.com",
		"dash_example.com",
	} {
		t.Run("reject "+input, func(t *testing.T) {
			if got, err := canonicalPublicDomain(input); err == nil {
				t.Fatalf("canonicalPublicDomain(%q) = %q, nil", input, got)
			}
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("output failed")
}
