package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yacobolo/leapview/internal/app/config"
	"github.com/Yacobolo/leapview/internal/manageddata/localplan"
	workspacecompiler "github.com/Yacobolo/leapview/internal/project/compiler"
)

func TestEvaluationCommandExposesServerAndOneTimeFirstLogin(t *testing.T) {
	command := evaluationCommand(context.Background(), &rootOptions{})
	if command.Name() != "evaluate" || !command.Runnable() {
		t.Fatalf("evaluation command = %#v, want runnable evaluate command", command)
	}
	firstLogin, _, err := command.Find([]string{"first-login"})
	if err != nil || firstLogin == nil || firstLogin.Name() != "first-login" {
		t.Fatalf("first-login command = %#v, err=%v", firstLogin, err)
	}
}

func TestConfigureEvaluationEnvironmentPersistsPrivateRuntimeSecrets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LEAPVIEW_HOME", home)
	t.Setenv("LEAPVIEW_PUBLIC_URL", "https://unsafe.example.com")
	t.Setenv("LEAPVIEW_TRUST_PROXY_HEADERS", "true")

	if err := configureEvaluationEnvironment(home); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.EvaluationMode || !cfg.Production || cfg.Environment != evaluationEnvironment ||
		!cfg.LocalAuth || cfg.PublicURL != evaluationPublicURL || cfg.TrustProxyHeaders {
		t.Fatalf("evaluation configuration = %#v", cfg)
	}
	if err := cfg.Validate(config.ProfileServe); err != nil {
		t.Fatalf("evaluation configuration validation: %v", err)
	}
	path := evaluationRuntimeConfigPath(home)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("runtime config mode = %o, want 600", info.Mode().Perm())
	}
	firstCSRF := cfg.CSRFKey
	if err := configureEvaluationEnvironment(home); err != nil {
		t.Fatal(err)
	}
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CSRFKey != firstCSRF {
		t.Fatal("evaluation runtime secret changed across restart")
	}
}

func TestEvaluationCredentialHandoffIsPrivateRecoverableAndOneTime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LEAPVIEW_HOME", home)
	if err := configureEvaluationEnvironment(home); err != nil {
		t.Fatal(err)
	}
	token, err := prepareEvaluationCredentials(context.Background(), home)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(token) == "" {
		t.Fatal("evaluation bootstrap token is empty")
	}
	if _, err := os.Stat(initialCredentialRecoveryPath(home)); !os.IsNotExist(err) {
		t.Fatalf("platform recovery bundle still exists: %v", err)
	}
	info, err := os.Stat(evaluationFirstLoginPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("first-login mode = %o, want 600", info.Mode().Perm())
	}

	var out bytes.Buffer
	if err := consumeEvaluationFirstLogin(home, &out); err != nil {
		t.Fatal(err)
	}
	var credentials initialInstanceCredentials
	if err := json.Unmarshal(out.Bytes(), &credentials); err != nil {
		t.Fatal(err)
	}
	if credentials.Email != evaluationAdminEmail || credentials.TemporaryPassword == "" {
		t.Fatalf("first-login credentials = %#v", credentials)
	}
	if _, err := os.Stat(evaluationFirstLoginPath(home)); !os.IsNotExist(err) {
		t.Fatalf("first-login file remains after successful delivery: %v", err)
	}
	if got, err := readEvaluationBootstrapToken(home); err != nil || got != token {
		t.Fatalf("bootstrap token after first-login = %q, %v", got, err)
	}
	if err := consumeEvaluationFirstLogin(home, &out); err == nil {
		t.Fatal("second first-login delivery succeeded")
	}
}

func TestEvaluationFirstLoginRetainedWhenDeliveryFails(t *testing.T) {
	home := t.TempDir()
	contents := []byte(`{"email":"admin@localhost","temporaryPassword":"temporary","publisherToken":"publisher","publisherTokenExpiresAt":"2026-07-28T00:00:00Z"}` + "\n")
	if err := writeInitialCredentialRecovery(evaluationFirstLoginPath(home), contents); err != nil {
		t.Fatal(err)
	}
	if err := consumeEvaluationFirstLogin(home, errorWriter{}); err == nil {
		t.Fatal("first-login output failure = nil")
	}
	if _, err := os.Stat(filepath.Join(home, evaluationFirstLoginFileName)); err != nil {
		t.Fatalf("first-login credentials not retained: %v", err)
	}
}

func TestBundledEvaluationProjectCompilesAndPlansOneSmallManagedFile(t *testing.T) {
	root, err := evaluationAssetsRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(root, evaluationProjectRelativePath)
	compiled, err := workspacecompiler.CompileProject(projectPath, workspacecompiler.Options{ServingStateID: "evaluation-test"})
	if err != nil {
		t.Fatal(err)
	}
	if got := compiled.WorkspaceIDs(); len(got) != 1 || got[0] != evaluationWorkspaceID {
		t.Fatalf("compiled evaluation workspaces = %#v", got)
	}
	plan, err := localplan.NewService(loadManagedDataPlanProject).Plan(context.Background(), localplan.Request{
		ProjectPath: projectPath,
		Connection:  evaluationConnection,
		From:        filepath.Join(root, evaluationDataRelativePath),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Manifest.Files) != 1 || plan.Manifest.Files[0].Path != "orders.csv" || plan.Manifest.Files[0].Size > 16<<10 {
		t.Fatalf("evaluation manifest = %#v", plan.Manifest)
	}
}

func TestEvaluationCompletionMarkerIsStrictAndPrivate(t *testing.T) {
	home := t.TempDir()
	completion := evaluationCompletion{
		ProjectID:  evaluationProjectID,
		Workspace:  evaluationWorkspaceID,
		Dashboard:  evaluationDashboardID,
		RevisionID: "sha256:" + strings.Repeat("a", 64),
	}
	if err := writeEvaluationCompletion(home, completion); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(evaluationCompletePath(home))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("completion mode = %o, want 600", info.Mode().Perm())
	}
	got, err := readEvaluationCompletion(home)
	if err != nil || got != completion {
		t.Fatalf("completion = %#v, %v", got, err)
	}
}
