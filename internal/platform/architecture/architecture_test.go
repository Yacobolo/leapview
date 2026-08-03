package architecture

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

const modulePath = "github.com/flidai/leapview"

type goFile struct {
	path    string
	pkgDir  string
	imports []string
	body    string
}

var targetCapabilities = map[string]struct{}{
	"project": {}, "workspace": {}, "access": {}, "manageddata": {}, "analytics": {},
	"dashboard": {}, "agent": {}, "release": {}, "deployment": {}, "servingstate": {},
	"refresh": {}, "runtimehost": {}, "workload": {}, "platform": {},
}

var approvedInternalRoots = map[string]struct{}{
	"app": {}, "platform": {},
	"access": {}, "admin": {}, "agent": {}, "analytics": {}, "dashboard": {},
	"deployment": {}, "manageddata": {}, "project": {}, "refresh": {}, "release": {},
	"runtimehost": {}, "servingstate": {}, "workload": {}, "workspace": {},
}

func TestRepositoryIdentityUsesOrganizationNamespace(t *testing.T) {
	const canonicalModule = "github.com/flidai/leapview"
	if modulePath != canonicalModule {
		t.Errorf("modulePath = %q, want %q", modulePath, canonicalModule)
	}

	goModule, err := os.ReadFile(filepath.Join(repoRoot(t), "go.mod"))
	require.NoError(t, err)
	if !strings.HasPrefix(string(goModule), "module "+canonicalModule+"\n") {
		t.Errorf("go.mod does not declare %s", canonicalModule)
	}

	legacyRepository := "github.com/" + "Yacobolo" + "/leapview"
	legacyImages := "ghcr.io/" + "yacobolo" + "/leapview"
	legacyImageAllowlist := map[string]struct{}{
		"docs/articles/start/installation.md": {},
		"docs/public-release.json":            {},
		"scripts/public_site_smoke.test.ts":   {},
	}
	root := repoRoot(t)
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".data", ".git", ".leapview", ".terraform", ".tmp", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		// Packaged applications contain directory symlinks. Repository identity
		// is enforced against authored regular files, not generated filesystem
		// topology or other special entries.
		if !entry.Type().IsRegular() {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !utf8.Valid(body) {
			return nil
		}
		text := string(body)
		for _, forbidden := range []string{legacyRepository, legacyImages} {
			if strings.Contains(text, forbidden) {
				relative, relErr := filepath.Rel(root, path)
				if relErr != nil {
					return relErr
				}
				relativePath := filepath.ToSlash(relative)
				if forbidden == legacyImages {
					if _, allowed := legacyImageAllowlist[relativePath]; allowed {
						continue
					}
				}
				t.Errorf("%s retains legacy repository namespace %q", relativePath, forbidden)
			}
		}
		return nil
	})
	require.NoError(t, err)
}

func TestInternalRootTaxonomyIsClosed(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join(repoRoot(t), "internal"))
	require.NoError(t, err)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, ok := approvedInternalRoots[entry.Name()]; !ok {
			t.Errorf("internal/%s is outside the approved root taxonomy", entry.Name())
		}
	}
}

func TestArchitectureOwnershipUsesRootTaxonomy(t *testing.T) {
	for _, rule := range PackageRules {
		if rule.Capability == "api" || rule.Capability == "ui" {
			t.Errorf("%s retains synthetic %q ownership instead of its physical app, platform, or capability owner", rule.Prefix, rule.Capability)
		}
	}
}

func TestAgentGeneratedAPIIsCapabilityOwned(t *testing.T) {
	rule, ok := ClassifyPackage("internal/agent/api/gen")
	if !ok {
		t.Fatal("Agent generated API package is not classified")
	}
	if rule.Capability != "agent" || rule.Layer != LayerAdapter {
		t.Fatalf("Agent generated API classification = %#v, want agent adapter", rule)
	}
	aggregate, ok := ClassifyPackage("internal/app/api/aggregate")
	if !ok || aggregate.Capability != "composition" || aggregate.Layer != LayerAdapter {
		t.Fatalf("application API aggregate classification = %#v, want composition adapter", aggregate)
	}
}

func TestCapabilityCLIIsAnAdapterOwnedByItsCapability(t *testing.T) {
	for _, capability := range []string{"access", "admin", "agent", "dashboard", "manageddata", "project", "workspace"} {
		rule, ok := ClassifyPackage("internal/" + capability + "/cli")
		if !ok {
			t.Fatalf("%s CLI package is not classified", capability)
		}
		if rule.Capability != capability || rule.Layer != LayerAdapter {
			t.Fatalf("%s CLI classification = %#v, want %s adapter", capability, rule, capability)
		}
	}
}

func TestEnterpriseAuthoringPackagesRemainCapabilityOwned(t *testing.T) {
	tests := []struct {
		path       string
		capability string
		layer      Layer
	}{
		{path: "internal/platform/securestore", capability: "platform", layer: LayerPlatform},
		{path: "internal/access/cli", capability: "access", layer: LayerAdapter},
		{path: "internal/project/devloop", capability: "project", layer: LayerUseCase},
		{path: "internal/analytics/connectionbinding", capability: "analytics", layer: LayerUseCase},
		{path: "internal/analytics/infisical", capability: "analytics", layer: LayerAdapter},
		{path: "internal/analytics/environment", capability: "analytics", layer: LayerAdapter},
		{path: "internal/analytics/sqlite", capability: "analytics", layer: LayerAdapter},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			rule, ok := ClassifyPackage(test.path)
			if !ok {
				t.Fatalf("%s is not classified", test.path)
			}
			if rule.Capability != test.capability || rule.Layer != test.layer {
				t.Fatalf("%s classification = %#v, want %s %s", test.path, rule, test.capability, test.layer)
			}
		})
	}
}

func TestEnterpriseAuthoringForbiddenImportsAreRejected(t *testing.T) {
	tests := []struct {
		name   string
		source string
		target string
	}{
		{
			name:   "project dev loop cannot import release adapters",
			source: "internal/project/devloop",
			target: "internal/release/filesystem",
		},
		{
			name:   "project dev loop cannot import deployment adapters",
			source: "internal/project/devloop",
			target: "internal/deployment/http",
		},
		{
			name:   "dashboard cannot import deployment",
			source: "internal/dashboard/runtime",
			target: "internal/deployment",
		},
		{
			name:   "workspace cannot import deployment",
			source: "internal/workspace",
			target: "internal/deployment",
		},
		{
			name:   "runtime host cannot import access",
			source: "internal/runtimehost",
			target: "internal/access",
		},
		{
			name:   "runtime host cannot import deployment",
			source: "internal/runtimehost",
			target: "internal/deployment",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source, sourceOK := ClassifyPackage(test.source)
			target, targetOK := ClassifyPackage(test.target)
			if !sourceOK || !targetOK {
				t.Fatalf("classify source=%s (%v) target=%s (%v)", test.source, sourceOK, test.target, targetOK)
			}
			if violation := CapabilityImportViolation(test.source, source, test.target, target); !strings.Contains(violation, "undeclared capability edge") {
				t.Fatalf("%s -> %s violation = %q, want undeclared capability edge", test.source, test.target, violation)
			}
		})
	}
}

func TestEnterpriseAuthoringStateRemainsCapabilityOwned(t *testing.T) {
	tests := []struct {
		path       string
		capability string
		layer      Layer
	}{
		{path: "internal/deployment", capability: "deployment", layer: LayerContract},
		{path: "internal/deployment/sqlite", capability: "deployment", layer: LayerAdapter},
		{path: "internal/release", capability: "release", layer: LayerContract},
		{path: "internal/release/filesystem", capability: "release", layer: LayerAdapter},
		{path: "internal/analytics/connectionbinding", capability: "analytics", layer: LayerUseCase},
		{path: "internal/analytics/infisical", capability: "analytics", layer: LayerAdapter},
		{path: "internal/analytics/environment", capability: "analytics", layer: LayerAdapter},
		{path: "internal/analytics/sqlite", capability: "analytics", layer: LayerAdapter},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			rule, ok := ClassifyPackage(test.path)
			if !ok {
				t.Fatalf("%s is not classified", test.path)
			}
			if rule.Capability != test.capability || rule.Layer != test.layer {
				t.Fatalf("%s classification = %#v, want %s %s", test.path, rule, test.capability, test.layer)
			}
		})
	}

	root := repoRoot(t)
	for _, forbidden := range []string{
		"internal/project/candidate",
		"internal/release/candidate",
		"internal/manageddata/connectionbinding",
	} {
		if packageDirExists(root, forbidden) {
			t.Errorf("%s claims enterprise-authoring state owned by another capability", forbidden)
		}
	}
}

func TestEnterpriseAuthoringCapabilityDirectionIsExplicit(t *testing.T) {
	required := map[string][]string{
		"project":     {"access", "analytics", "dashboard", "refresh", "servingstate", "workspace"},
		"release":     {"access", "project", "servingstate", "workspace"},
		"deployment":  {"access", "manageddata", "project", "release", "runtimehost", "servingstate"},
		"runtimehost": {"manageddata", "servingstate"},
	}
	for source, targets := range required {
		for _, target := range targets {
			if !CapabilityDependencies[source][target] {
				t.Errorf("enterprise authoring capability edge %s -> %s is not declared", source, target)
			}
		}
	}
	for source, forbidden := range map[string][]string{
		"access":      {"project", "release", "deployment", "runtimehost"},
		"project":     {"release", "deployment", "runtimehost"},
		"release":     {"deployment", "runtimehost"},
		"runtimehost": {"access", "project", "release", "deployment"},
	} {
		for _, target := range forbidden {
			if CapabilityDependencies[source][target] {
				t.Errorf("enterprise authoring capability graph permits reverse edge %s -> %s", source, target)
			}
		}
	}
}

func TestEnterpriseAuthoringGuideDefinesOneTargetHostedLifecycle(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "docs", "guides", "cli", "validate-deploy.md")
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(body)
	for _, required := range []string{
		"already-running LeapView target",
		"leapview login",
		"leapview dev",
		"leapview publish",
		"localhost",
		"hosted",
		"self-hosted",
		"air-gapped",
		"synthetic data",
		"operator bootstrap",
		"canonical-origin, token-free HTTPS URL",
		"system browser by default",
		"does not require LeapView Desktop",
		"HttpOnly",
		"independently revocable",
		"row-level security",
		"read-only Infisical",
		"bounded stale",
		"already-authenticated source sessions",
		"built-in vault",
		"dynamic leases",
		"Kubernetes integration",
		"Capability",
		"Dependency direction",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("canonical enterprise authoring guide missing %q", required)
		}
	}
	login := strings.Index(text, "leapview login")
	dev := strings.Index(text, "leapview dev")
	publish := strings.Index(text, "leapview publish")
	if login < 0 || dev <= login || publish <= dev {
		t.Errorf("canonical commands are not taught in login -> dev -> publish order")
	}

	guideDirectory := filepath.Join(root, "docs", "guides", "cli")
	entries, err := os.ReadDir(guideDirectory)
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(guideDirectory, entry.Name()))
		require.NoError(t, err)
		for _, forbidden := range []string{
			"leapview deploy",
			"leapview preview",
			"leapview staging",
			"--auto-approve",
		} {
			if strings.Contains(string(content), forbidden) {
				t.Errorf("docs/guides/cli/%s presents alternate authoring command %q", entry.Name(), forbidden)
			}
		}
	}
}

func TestCapabilityCLIsUseGeneratedTypedClients(t *testing.T) {
	clientImports := map[string]string{
		"internal/agent/cli":     modulePath + "/internal/agent/api/gen",
		"internal/dashboard/cli": modulePath + "/internal/dashboard/api/gen",
		"internal/workspace/cli": modulePath + "/internal/workspace/api/gen",
	}
	seen := map[string]bool{}
	for _, file := range productionGoFiles(t) {
		requiredImport, capabilityCLI := clientImports[file.pkgDir]
		if !capabilityCLI {
			continue
		}
		seen[file.pkgDir] = seen[file.pkgDir] || importListContains(file.imports, requiredImport)
		for _, forbidden := range []string{"cliapi.Request", ".DoJSON(", `OperationID: "`} {
			if strings.Contains(file.body, forbidden) {
				t.Errorf("%s retains untyped CLI API surface %q", file.path, forbidden)
			}
		}
	}
	for pkgDir := range clientImports {
		if !seen[pkgDir] {
			t.Errorf("%s does not import its generated typed client package", pkgDir)
		}
	}

	cliAPI, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "platform", "cliapi", "client.go"))
	require.NoError(t, err)
	for _, forbidden := range []string{"type Request struct", "DoJSON("} {
		if strings.Contains(string(cliAPI), forbidden) {
			t.Errorf("platform CLI port retains transitional surface %q", forbidden)
		}
	}
}

func TestAccessCLIUsesStandardOAuthClient(t *testing.T) {
	requiredImports := map[string]bool{
		"golang.org/x/oauth2":                   false,
		"golang.org/x/oauth2/clientcredentials": false,
	}
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/access/cli" {
			continue
		}
		for requiredImport := range requiredImports {
			requiredImports[requiredImport] = requiredImports[requiredImport] ||
				importListContains(file.imports, requiredImport)
		}
		if importListContains(file.imports, modulePath+"/internal/access/api/gen") {
			t.Errorf("%s routes OAuth lifecycle operations through the generated REST client", file.path)
		}
	}
	for requiredImport, found := range requiredImports {
		if !found {
			t.Errorf("internal/access/cli does not import standard OAuth package %q", requiredImport)
		}
	}
}

func TestProductionCodeDoesNotImportTestcontainers(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		for _, imported := range file.imports {
			if strings.HasPrefix(imported, "github.com/testcontainers/testcontainers-go") {
				t.Errorf("%s imports test-only container framework %q", file.path, imported)
			}
		}
	}
}

func TestMinIOIntegrationOwnsItsContainerLifecycle(t *testing.T) {
	root := repoRoot(t)
	testSource, err := os.ReadFile(filepath.Join(root, "internal", "app", "integration_minio_source_test.go"))
	require.NoError(t, err)
	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	require.NoError(t, err)
	taskfile, err := os.ReadFile(filepath.Join(root, "Taskfile.yml"))
	require.NoError(t, err)

	testText := string(testSource)
	for _, want := range []string{
		`github.com/testcontainers/testcontainers-go/modules/minio`,
		`testcontainers.CleanupContainer(t, minioContainer)`,
		`testcontainers.WithLogger(log.TestLogger(t))`,
		`minio/minio@sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e`,
	} {
		if !strings.Contains(testText, want) {
			t.Errorf("MinIO integration test must own container lifecycle: missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"LEAPVIEW_TEST_MINIO_ENDPOINT",
		"Start MinIO source integration service",
		"docker run --detach --name leapview-minio",
		"minio/minio@sha256:",
	} {
		if strings.Contains(string(workflow), forbidden) {
			t.Errorf("CI workflow must not own MinIO integration lifecycle: found %q", forbidden)
		}
	}
	taskText := string(taskfile)
	for _, want := range []string{
		"test:go:external:",
		`-run '^TestMinIOParquetSourceRefreshContract$'`,
	} {
		if !strings.Contains(taskText, want) {
			t.Errorf("local Go suite must run external-service tests serially: missing %q", want)
		}
	}
	const skipMinIO = `-skip '^TestMinIOParquetSourceRefreshContract$'`
	if count := strings.Count(taskText, skipMinIO); count != 4 {
		t.Errorf("each local application shard must defer MinIO to the serial external-service task: found %d %q flags, want 4", count, skipMinIO)
	}
}

func TestCapabilityAPIPackagesOptIntoTypedClientGeneration(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(repoRoot(t), "api", "apigen.yaml"))
	require.NoError(t, err)
	manifest := string(content)
	namespaces := []string{
		"LeapViewAPI.Access", "LeapViewAPI.Agent", "LeapViewAPI.Analytics",
		"LeapViewAPI.Dashboard", "LeapViewAPI.Deployment", "LeapViewAPI.ManagedData",
		"LeapViewAPI.Project", "LeapViewAPI.Refresh", "LeapViewAPI.Release",
		"LeapViewAPI.Workspace",
	}
	for _, namespace := range namespaces {
		start := strings.Index(manifest, "        "+namespace+":")
		if start < 0 {
			t.Errorf("APIGen manifest is missing %s", namespace)
			continue
		}
		rest := manifest[start+1:]
		end := strings.Index(rest, "\n        LeapViewAPI.")
		if end >= 0 {
			rest = rest[:end]
		}
		if !strings.Contains(rest, "client_file: client.apigen.gen.go") {
			t.Errorf("%s does not own a generated typed client", namespace)
		}
	}
}

func TestApplicationCLIAdminOnlyComposesAdminOperations(t *testing.T) {
	forbiddenImports := map[string]bool{
		modulePath + "/internal/access/sqlite":       true,
		modulePath + "/internal/admin/sqlite":        true,
		modulePath + "/internal/analytics/ducklake":  true,
		modulePath + "/internal/servingstate/sqlite": true,
	}
	var adminFile *goFile
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/app/cli" {
			continue
		}
		for _, imported := range file.imports {
			if forbiddenImports[imported] {
				t.Errorf("%s imports offline capability adapter %s", file.path, imported)
			}
		}
		if file.path == "internal/app/cli/admin.go" {
			current := file
			adminFile = &current
		}
	}
	if adminFile == nil {
		t.Fatal("internal/app/cli/admin.go was not found")
	}
	for _, required := range []string{
		modulePath + "/internal/admin/cli",
		modulePath + "/internal/app/adminoffline",
	} {
		if !importListContains(adminFile.imports, required) {
			t.Errorf("application CLI Admin composition is missing import %s", required)
		}
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), adminFile.path, adminFile.body, 0)
	require.NoError(t, err)
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if function.Name.Name != "adminCommand" {
			t.Errorf("application CLI Admin composition retains compatibility function %s", function.Name.Name)
		}
	}
}

func TestOfflineAdminUseCasesAreCapabilityOwned(t *testing.T) {
	rule, ok := ClassifyPackage("internal/admin/offline")
	if !ok {
		t.Fatal("Admin offline package is not classified")
	}
	if rule.Capability != "admin" || rule.Layer != LayerUseCase {
		t.Fatalf("Admin offline classification = %#v, want admin use-case", rule)
	}

	forbiddenImports := map[string]bool{
		modulePath + "/internal/access/sqlite":          true,
		modulePath + "/internal/admin/sqlite":           true,
		modulePath + "/internal/analytics/ducklake":     true,
		modulePath + "/internal/app/config":             true,
		modulePath + "/internal/platform":               true,
		modulePath + "/internal/platform/locking":       true,
		modulePath + "/internal/servingstate/retention": true,
		modulePath + "/internal/servingstate/sqlite":    true,
	}
	found := false
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/admin/offline" {
			continue
		}
		found = true
		for _, imported := range file.imports {
			if forbiddenImports[imported] {
				t.Errorf("%s imports concrete application/infrastructure dependency %s", file.path, imported)
			}
		}
	}
	if !found {
		t.Fatal("internal/admin/offline production package was not found")
	}

	compositionFound := false
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/app/adminoffline" {
			continue
		}
		compositionFound = true
		if !importListContains(file.imports, modulePath+"/internal/admin/offline") {
			t.Errorf("%s does not compose Admin-owned offline use cases", file.path)
		}
		for _, forbidden := range []string{
			"mail.ParseAddress(",
			"json.Marshal(",
			"fmt.Fprintf(",
			"retention days must be zero or greater",
			"admin restore requires --confirm",
			"admin backup requires --out",
		} {
			if strings.Contains(file.body, forbidden) {
				t.Errorf("%s retains offline Admin product behavior %q", file.path, forbidden)
			}
		}
	}
	if !compositionFound {
		t.Fatal("internal/app/adminoffline composition package was not found")
	}
}

func TestAccessGeneratedAPIIsCapabilityOwned(t *testing.T) {
	rule, ok := ClassifyPackage("internal/access/api/gen")
	if !ok {
		t.Fatal("Access generated API package is not classified")
	}
	if rule.Capability != "access" || rule.Layer != LayerAdapter {
		t.Fatalf("Access generated API classification = %#v, want access adapter", rule)
	}
}

func TestAnalyticsGeneratedAPIIsCapabilityOwned(t *testing.T) {
	rule, ok := ClassifyPackage("internal/analytics/api/gen")
	if !ok {
		t.Fatal("Analytics generated API package is not classified")
	}
	if rule.Capability != "analytics" || rule.Layer != LayerAdapter {
		t.Fatalf("Analytics generated API classification = %#v, want analytics adapter", rule)
	}
}

func TestProjectGeneratedAPIIsCapabilityOwned(t *testing.T) {
	rule, ok := ClassifyPackage("internal/project/api/gen")
	if !ok {
		t.Fatal("Project generated API package is not classified")
	}
	if rule.Capability != "project" || rule.Layer != LayerAdapter {
		t.Fatalf("Project generated API classification = %#v, want project adapter", rule)
	}
}

func TestProjectTransportContractsAreCapabilityOwned(t *testing.T) {
	root := repoRoot(t)
	projectContracts, err := os.ReadFile(filepath.Join(root, "internal", "project", "api", "contracts.go"))
	if err != nil {
		t.Fatalf("read Project API contracts: %v", err)
	}
	if !strings.Contains(string(projectContracts), "type ProjectResponse struct") {
		t.Fatal("Project capability does not own its handwritten response contract")
	}
	releaseContracts, err := os.ReadFile(filepath.Join(root, "internal", "release", "api", "contracts.go"))
	if err != nil {
		t.Fatalf("read Release API contracts: %v", err)
	}
	if strings.Contains(string(releaseContracts), "type ProjectResponse struct") {
		t.Fatal("Release capability still owns the Project response contract")
	}
}

func TestRefreshGeneratedAPIIsCapabilityOwned(t *testing.T) {
	rule, ok := ClassifyPackage("internal/refresh/api/gen")
	if !ok {
		t.Fatal("Refresh generated API package is not classified")
	}
	if rule.Capability != "refresh" || rule.Layer != LayerAdapter {
		t.Fatalf("Refresh generated API classification = %#v, want refresh adapter", rule)
	}
}

func TestDeploymentGeneratedAPIIsCapabilityOwned(t *testing.T) {
	rule, ok := ClassifyPackage("internal/deployment/api/gen")
	if !ok {
		t.Fatal("Deployment generated API package is not classified")
	}
	if rule.Capability != "deployment" || rule.Layer != LayerAdapter {
		t.Fatalf("Deployment generated API classification = %#v, want deployment adapter", rule)
	}
}

func TestReleaseGeneratedAPIIsCapabilityOwned(t *testing.T) {
	rule, ok := ClassifyPackage("internal/release/api/gen")
	if !ok {
		t.Fatal("Release generated API package is not classified")
	}
	if rule.Capability != "release" || rule.Layer != LayerAdapter {
		t.Fatalf("Release generated API classification = %#v, want release adapter", rule)
	}
}

func TestWorkspaceGeneratedAPIIsCapabilityOwned(t *testing.T) {
	rule, ok := ClassifyPackage("internal/workspace/api/gen")
	if !ok {
		t.Fatal("Workspace generated API package is not classified")
	}
	if rule.Capability != "workspace" || rule.Layer != LayerAdapter {
		t.Fatalf("Workspace generated API classification = %#v, want workspace adapter", rule)
	}
}

func TestManagedDataGeneratedAPIIsCapabilityOwned(t *testing.T) {
	rule, ok := ClassifyPackage("internal/manageddata/api/gen")
	if !ok {
		t.Fatal("ManagedData generated API package is not classified")
	}
	if rule.Capability != "manageddata" || rule.Layer != LayerAdapter {
		t.Fatalf("ManagedData generated API classification = %#v, want manageddata adapter", rule)
	}
}

func TestDashboardGeneratedAPIIsCapabilityOwned(t *testing.T) {
	rule, ok := ClassifyPackage("internal/dashboard/api/gen")
	if !ok {
		t.Fatal("Dashboard generated API package is not classified")
	}
	if rule.Capability != "dashboard" || rule.Layer != LayerAdapter {
		t.Fatalf("Dashboard generated API classification = %#v, want dashboard adapter", rule)
	}
}

func TestApplicationOwnsProductConfigurationContract(t *testing.T) {
	root := repoRoot(t)
	if !packageDirExists(root, "internal/app/config/spec") {
		t.Fatal("application configuration contract is missing")
	}
	if packageDirExists(root, "internal/platform/config/spec") {
		t.Fatal("platform retains the product configuration contract")
	}
	for _, file := range productionGoFiles(t) {
		if (file.pkgDir == "internal/platform" || strings.HasPrefix(file.pkgDir, "internal/platform/")) &&
			strings.Contains(file.body, "DefaultWorkspaceID") {
			t.Errorf("%s retains the application default workspace setting", file.path)
		}
	}
}

func TestPlatformProductionCodeDoesNotOwnApplicationEnvironment(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/platform" && !strings.HasPrefix(file.pkgDir, "internal/platform/") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file.path, file.body, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file.path, err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			switch value := node.(type) {
			case *ast.CallExpr:
				selector, ok := value.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				packageName, ok := selector.X.(*ast.Ident)
				if ok && packageName.Name == "os" && (selector.Sel.Name == "Getenv" || selector.Sel.Name == "LookupEnv") {
					t.Errorf("%s reads the process environment directly; application composition must inject configuration", file.path)
				}
			case *ast.BasicLit:
				if strings.Contains(value.Value, "LEAPVIEW_") {
					t.Errorf("%s contains application-specific configuration %s", file.path, value.Value)
				}
			}
			return true
		})
	}
}

func TestPlatformProductionCodeDoesNotImportProductCapabilities(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/platform" && !strings.HasPrefix(file.pkgDir, "internal/platform/") {
			continue
		}
		for _, imported := range file.imports {
			if !strings.HasPrefix(imported, modulePath+"/internal/") {
				continue
			}
			targetPath := strings.TrimPrefix(imported, modulePath+"/")
			targetRoot := strings.Split(strings.TrimPrefix(targetPath, "internal/"), "/")[0]
			if targetRoot != "platform" {
				t.Errorf("%s imports product/app package %s", file.path, targetPath)
			}
		}
	}
}

func TestPlatformObservabilityContainsOnlyGenericMechanisms(t *testing.T) {
	root := repoRoot(t)
	if !packageDirExists(root, "internal/dashboard/observability") {
		t.Error("dashboard telemetry adapter is not owned by dashboard")
	}
	if !packageDirExists(root, "internal/workload/observability") {
		t.Error("workload telemetry adapter is not owned by workload")
	}
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/platform/observability" {
			continue
		}
		for _, productTerm := range []string{"Dashboard", "Workspace", "Workload", "Analytics", "ServingState"} {
			if strings.Contains(file.body, productTerm) {
				t.Errorf("%s contains product-owned observability term %q", file.path, productTerm)
			}
		}
	}
}

func TestCapabilitiesDoNotImportApplicationComposition(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		if !strings.HasPrefix(file.pkgDir, "internal/") ||
			file.pkgDir == "internal/app" || strings.HasPrefix(file.pkgDir, "internal/app/") ||
			file.pkgDir == "internal/platform" || strings.HasPrefix(file.pkgDir, "internal/platform/") {
			continue
		}
		for _, imported := range file.imports {
			if imported == modulePath+"/internal/app" || strings.HasPrefix(imported, modulePath+"/internal/app/") {
				t.Errorf("%s imports application composition package %s", file.path, imported)
			}
		}
	}
}

func TestDeferredCapabilityEdgesRemainEmpty(t *testing.T) {
	if len(DeferredPackageEdges) != 0 {
		t.Fatalf("deferred capability edges = %v, want none", DeferredPackageEdges)
	}
}

func TestTargetCapabilityGraphDeclaresWorkload(t *testing.T) {
	if _, ok := targetCapabilities["workload"]; !ok {
		t.Fatal("workload is absent from the target capability graph")
	}
	if !packageDirExists(repoRoot(t), "internal/workload") {
		t.Fatal("declared workload capability package does not exist")
	}
}

func TestRefreshOwnsDurableRunState(t *testing.T) {
	if !packageDirExists(repoRoot(t), "internal/refresh/run") {
		t.Fatal("refresh run contract package does not exist")
	}
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/analytics/materialize" {
			continue
		}
		for _, declaration := range []string{"type RunRecord struct", "type RunInput struct", "RunStatusQueued"} {
			if strings.Contains(file.body, declaration) {
				t.Errorf("%s retains refresh lifecycle declaration %q", file.path, declaration)
			}
		}
	}
}

func TestCapabilityModuleSurfacesExist(t *testing.T) {
	root := repoRoot(t)
	for _, capability := range []string{"access", "analytics", "workspace", "manageddata", "release", "deployment", "refresh", "dashboard", "agent", "runtimehost", "servingstate", "workload", "admin"} {
		dir := "internal/" + capability + "/module"
		if !packageDirExists(root, dir) {
			t.Errorf("capability composition package %s does not exist", dir)
		}
	}
}

func TestCapabilityModulesUseBuildAsTheirConstructor(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		rule, ok := ClassifyPackage(file.pkgDir)
		if !ok || rule.Layer != LayerModule {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file.path, file.body, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file.path, err)
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && function.Name.Name == "New" {
				t.Errorf("%s exports New; capability modules expose Build(ctx, Config)", file.path)
			}
		}
	}
}

func TestCapabilityModulesDoNotExposeRepositories(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		rule, ok := ClassifyPackage(file.pkgDir)
		if !ok || rule.Layer != LayerModule {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file.path, file.body, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file.path, err)
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv == nil {
				continue
			}
			if function.Name.Name == "Repository" {
				t.Errorf("%s exposes a repository from a capability module; export a named read or write port", file.path)
			}
		}
	}
}

func TestApplicationAPIGenRoutesUseGeneratedAggregate(t *testing.T) {
	foundAggregateRegistration := false
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/app" {
			continue
		}
		for _, forbidden := range []string{
			"apigenOperationPrivilege",
			"apigenOperationObjectResolver",
			"apiGenObjectScopes",
			"isGlobalAgentOperation",
		} {
			if strings.Contains(file.body, forbidden) {
				t.Errorf("%s retains APIGen authorization behavior %q; access owns authorization", file.path, forbidden)
			}
		}
		if strings.Contains(file.body, "apiaggregate.RegisterAPIGenRoutes(r, platform.apiGenServers)") {
			foundAggregateRegistration = true
		}
		if strings.Contains(file.body, "type apiGenRouteHandler") {
			t.Errorf("%s retains the handwritten global APIGen route wrapper", file.path)
		}
	}
	if !foundAggregateRegistration {
		t.Fatal("internal/app does not register the generated APIGen aggregate")
	}
}

func TestApplicationHasNoServerShapedDependencyContainer(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/app" {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file.path, file.body, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", file.path, err)
		}
		localStructs := map[string]*ast.StructType{}
		for _, declaration := range parsed.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.TYPE {
				continue
			}
			for _, spec := range generic.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if structure, ok := typeSpec.Type.(*ast.StructType); ok {
					localStructs[typeSpec.Name.Name] = structure
				}
			}
		}
		for name, structure := range localStructs {
			switch name {
			case "runtimeRouter", "assemblyConfig", "capabilityConstruction", "applicationAssembly", "assemblyInputs", "moduleAssemblyInputs":
				t.Errorf("%s retains transitional dependency container %s", file.path, name)
			}
			fields := expandedStructFieldCount(structure, localStructs, map[string]bool{name: true})
			if fields > 12 {
				t.Errorf("%s struct %s has %d transitive fields; split composition state into narrow route, lifecycle, health, and cleanup surfaces", file.path, name, fields)
			}
		}
	}
}

func expandedStructFieldCount(structure *ast.StructType, localStructs map[string]*ast.StructType, visiting map[string]bool) int {
	fields := 0
	for _, field := range structure.Fields.List {
		fieldCount := len(field.Names)
		if fieldCount == 0 {
			fieldCount = 1
		}
		identifier, ok := localStructIdentifier(field.Type)
		if !ok {
			fields += fieldCount
			continue
		}
		embedded, ok := localStructs[identifier.Name]
		if !ok || visiting[identifier.Name] {
			fields += fieldCount
			continue
		}
		visiting[identifier.Name] = true
		fields += fieldCount * expandedStructFieldCount(embedded, localStructs, visiting)
		delete(visiting, identifier.Name)
	}
	return fields
}

func localStructIdentifier(expression ast.Expr) (*ast.Ident, bool) {
	switch value := expression.(type) {
	case *ast.Ident:
		return value, true
	case *ast.StarExpr:
		identifier, ok := value.X.(*ast.Ident)
		return identifier, ok
	default:
		return nil, false
	}
}

func TestGeneratedQueryPackagesDoNotCombineCapabilitySQL(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "sqlc.yaml"))
	require.NoError(t, err)
	blocks := strings.Split(string(body), "\n  - engine:")
	for _, forbidden := range []struct {
		generatedPackage string
		queryPath        string
	}{
		{generatedPackage: `out: "internal/deployment/internal/db"`, queryPath: `"internal/servingstate/sqlite/queries`},
		{generatedPackage: `out: "internal/servingstate/internal/db"`, queryPath: `"internal/access/sqlite/queries`},
	} {
		for _, block := range blocks {
			if strings.Contains(block, forbidden.generatedPackage) && strings.Contains(block, forbidden.queryPath) {
				t.Errorf("sqlc package %s includes cross-capability query input %s", forbidden.generatedPackage, forbidden.queryPath)
			}
		}
	}
}

func TestCapabilitySQLCOutputsArePrivate(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "sqlc.yaml"))
	require.NoError(t, err)
	config := string(body)
	for _, output := range []string{
		"internal/access/internal/db",
		"internal/admin/internal/db",
		"internal/agent/internal/db",
		"internal/analytics/internal/db",
		"internal/dashboard/internal/db",
		"internal/deployment/internal/db",
		"internal/manageddata/internal/db",
		"internal/refresh/internal/db",
		"internal/release/internal/db",
		"internal/servingstate/internal/db",
		"internal/workspace/internal/db",
	} {
		fragment := "package: \"db\"\n        out: \"" + output + "\""
		if !strings.Contains(config, fragment) {
			t.Errorf("sqlc output %s is not a capability-private db package", output)
		}
	}
	for _, legacy := range []string{
		"internal/access/sqlite/accessdb",
		"internal/admin/sqlite/retentiondb",
		"internal/agent/sqlite/agentdb",
		"internal/analytics/queryaudit/sqlite/querydb",
		"internal/deployment/sqlite/deploymentdb",
		"internal/manageddata/sqlite/manageddb",
		"internal/refresh/sqlite/materializedb",
		"internal/refresh/sqlite/refreshdb",
		"internal/release/sqlite/releasedb",
		"internal/servingstate/sqlite/servingdb",
		"internal/workspace/sqlite/workspacedb",
	} {
		if strings.Contains(config, legacy) {
			t.Errorf("sqlc retains public capability output %s", legacy)
		}
	}
}

func TestCapabilitiesOnlyImportOwnGeneratedQueries(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		source, sourceOK := ClassifyPackage(file.pkgDir)
		for _, imported := range file.imports {
			targetOwner, generated := capabilityGeneratedDBOwner(imported)
			if !generated {
				continue
			}
			if !sourceOK || source.Capability != targetOwner || source.Layer != LayerAdapter {
				t.Errorf("%s imports generated database package owned by %s outside its owning persistence adapters", file.path, targetOwner)
			}
		}
	}
}

func TestCrossCapabilityGeneratedQueryImportIsRejected(t *testing.T) {
	if owner, ok := capabilityGeneratedDBOwner(modulePath + "/internal/access/internal/db"); !ok || owner != "access" {
		t.Fatalf("generated Access database package owner = %q, %v", owner, ok)
	}
	source, ok := ClassifyPackage("internal/dashboard/publication/sqlite")
	if !ok {
		t.Fatal("Dashboard publication adapter is not classified")
	}
	targetOwner, generated := capabilityGeneratedDBOwner(modulePath + "/internal/access/internal/db")
	if !generated || source.Capability == targetOwner {
		t.Fatal("representative Dashboard-to-Access generated database import was not rejected")
	}
}

func capabilityGeneratedDBOwner(imported string) (string, bool) {
	relative := strings.TrimPrefix(imported, modulePath+"/")
	parts := strings.Split(relative, "/")
	if len(parts) != 4 || parts[0] != "internal" || parts[2] != "internal" || parts[3] != "db" {
		return "", false
	}
	if _, known := CapabilityDependencies[parts[1]]; !known || parts[1] == "platform" {
		return "", false
	}
	return parts[1], true
}

func TestCompositionDoesNotUseTestTransports(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/app" {
			continue
		}
		for _, imported := range file.imports {
			if imported == "net/http/httptest" {
				t.Errorf("%s uses httptest in process composition; response capture belongs to the consuming transport adapter", file.path)
			}
		}
	}
}

func TestRefreshPersistenceIsConstructedOnlyByItsModule(t *testing.T) {
	constructors := 0
	for _, file := range productionGoFiles(t) {
		for _, imported := range file.imports {
			if imported != modulePath+"/internal/refresh/sqlite" {
				continue
			}
			if file.pkgDir != "internal/refresh/module" {
				t.Errorf("%s imports refresh persistence outside refresh/module", file.path)
			}
		}
		if file.pkgDir == "internal/refresh/module" {
			constructors += strings.Count(file.body, "refreshsqlite.NewSQLRunRepository(")
			constructors += strings.Count(file.body, "refreshsqlite.NewRepository(")
		}
	}
	if constructors != 3 {
		t.Fatalf("refresh/module persistence constructors = %d, want 3 (run, schedule, recovery)", constructors)
	}
}

func TestPlatformJobModuleSurfaceExists(t *testing.T) {
	if !packageDirExists(repoRoot(t), "internal/platform/jobs/module") {
		t.Fatal("platform durable job module does not exist")
	}
}

func TestCapabilityModulesDoNotImportOtherModules(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		source, ok := ClassifyPackage(file.pkgDir)
		if !ok || source.Layer != LayerModule {
			continue
		}
		for _, imported := range file.imports {
			if !strings.HasPrefix(imported, modulePath+"/internal/") || !strings.HasSuffix(imported, "/module") {
				continue
			}
			packagePath := strings.TrimPrefix(imported, modulePath+"/")
			target, ok := ClassifyPackage(packagePath)
			if ok && target.Capability != source.Capability {
				t.Errorf("%s imports capability module %s; only internal/app may assemble modules", file.path, packagePath)
			}
		}
	}
}

func TestCapabilityModulesDoNotImportOtherCapabilityAdapters(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		source, ok := ClassifyPackage(file.pkgDir)
		if !ok || source.Layer != LayerModule {
			continue
		}
		for _, imported := range file.imports {
			if !strings.HasPrefix(imported, modulePath+"/internal/") {
				continue
			}
			packagePath := strings.TrimPrefix(imported, modulePath+"/")
			target, ok := ClassifyPackage(packagePath)
			if !ok || target.Layer != LayerAdapter || target.Capability == source.Capability {
				continue
			}
			if target.Capability == "platform" || target.Capability == "api" || target.Capability == "ui" {
				continue
			}
			t.Errorf("%s imports another capability's adapter %s; accept a consumer-owned port", file.path, packagePath)
		}
	}
}

func TestCapabilityModulesDoNotImportOtherCapabilityPersistenceAdapters(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		source, ok := ClassifyPackage(file.pkgDir)
		if !ok || source.Layer != LayerModule {
			continue
		}
		for _, imported := range file.imports {
			if !strings.HasPrefix(imported, modulePath+"/internal/") {
				continue
			}
			packagePath := strings.TrimPrefix(imported, modulePath+"/")
			target, ok := ClassifyPackage(packagePath)
			if !ok || target.Layer != LayerAdapter || target.Capability == source.Capability || !strings.Contains(packagePath, "/sqlite") {
				continue
			}
			if target.Capability == "platform" || target.Capability == "api" || target.Capability == "ui" {
				continue
			}
			t.Errorf("%s imports another capability's adapter %s; receive a contract through Config instead", file.path, packagePath)
		}
	}
}

func TestCapabilityModulesDoNotImportOtherCapabilityTransportAdapters(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		source, ok := ClassifyPackage(file.pkgDir)
		if !ok || source.Layer != LayerModule {
			continue
		}
		for _, imported := range file.imports {
			if !strings.HasPrefix(imported, modulePath+"/internal/") {
				continue
			}
			packagePath := strings.TrimPrefix(imported, modulePath+"/")
			target, ok := ClassifyPackage(packagePath)
			if !ok || target.Layer != LayerAdapter || target.Capability == source.Capability {
				continue
			}
			if target.Capability == "platform" || target.Capability == "api" || target.Capability == "ui" {
				continue
			}
			if strings.Contains(packagePath, "/http") || strings.Contains(packagePath, "/datastar") {
				t.Errorf("%s imports another capability's transport adapter %s; accept a consumer-owned port", file.path, packagePath)
			}
		}
	}
}

func TestCompositionOwnershipIsAnExplicitClosedSet(t *testing.T) {
	allowed := []string{
		"cmd",
		"internal/app",
		"internal/app/cli",
		"internal/app/tools",
	}
	for _, file := range productionGoFiles(t) {
		rule, ok := ClassifyPackage(file.pkgDir)
		if !ok || rule.Layer != LayerComposition {
			continue
		}
		permitted := false
		for _, prefix := range allowed {
			if file.pkgDir == prefix || strings.HasPrefix(file.pkgDir, prefix+"/") {
				permitted = true
				break
			}
		}
		if !permitted {
			t.Errorf("%s claims undeclared composition ownership", file.path)
		}
	}
}

func TestEveryProductionPackageHasAnArchitecturalOwner(t *testing.T) {
	seen := map[string]bool{}
	for _, file := range productionGoFiles(t) {
		if seen[file.pkgDir] {
			continue
		}
		seen[file.pkgDir] = true
		if _, ok := ClassifyPackage(file.pkgDir); !ok {
			t.Errorf("%s has no declared capability owner and layer", file.pkgDir)
		}
	}
}

func TestDeclaredCapabilityGraphHasNoReciprocalEdges(t *testing.T) {
	for source, dependencies := range CapabilityDependencies {
		for target := range dependencies {
			if CapabilityDependencies[target][source] {
				t.Errorf("capability graph contains reciprocal edges %s -> %s and %s -> %s", source, target, target, source)
			}
		}
	}
}

func TestProductionImportsFollowCapabilityGraph(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		if strings.Contains(file.pkgDir, "/testing/") {
			continue
		}
		source, ok := ClassifyPackage(file.pkgDir)
		if !ok {
			continue
		}
		for _, imported := range file.imports {
			if !strings.HasPrefix(imported, modulePath+"/") {
				continue
			}
			packagePath := strings.TrimPrefix(imported, modulePath+"/")
			target, ok := ClassifyPackage(packagePath)
			if !ok || source.Capability == target.Capability {
				continue
			}
			_, sourceIsProductCapability := targetCapabilities[source.Capability]
			if !sourceIsProductCapability || source.Layer == LayerComposition {
				continue
			}
			if violation := CapabilityImportViolation(file.pkgDir, source, packagePath, target); violation != "" {
				t.Errorf("%s imports %s: %s", file.path, packagePath, violation)
			}
		}
	}
}

func TestCapabilityModulesRequireDeclaredPublicContractEdges(t *testing.T) {
	runtimehostModule, ok := ClassifyPackage("internal/runtimehost/module")
	if !ok || runtimehostModule.Layer != LayerModule {
		t.Fatal("runtimehost module classification is unavailable")
	}
	projectBundle, ok := ClassifyPackage("internal/project/bundle")
	if !ok {
		t.Fatal("project bundle classification is unavailable")
	}
	if violation := CapabilityImportViolation("internal/runtimehost/module", runtimehostModule, "internal/project/bundle", projectBundle); !strings.Contains(violation, "undeclared capability edge") {
		t.Fatalf("runtimehost module -> project bundle violation = %q", violation)
	}

	agentModule, ok := ClassifyPackage("internal/agent/module")
	if !ok || agentModule.Layer != LayerModule {
		t.Fatal("agent module classification is unavailable")
	}
	dashboardRuntime, ok := ClassifyPackage("internal/dashboard/runtime")
	if !ok {
		t.Fatal("dashboard runtime classification is unavailable")
	}
	if violation := CapabilityImportViolation("internal/agent/module", agentModule, "internal/dashboard/runtime", dashboardRuntime); !strings.Contains(violation, "non-contract package") {
		t.Fatalf("agent module -> dashboard runtime violation = %q", violation)
	}

	dashboardReport, ok := ClassifyPackage("internal/dashboard/report")
	if !ok {
		t.Fatal("dashboard report classification is unavailable")
	}
	if violation := CapabilityImportViolation("internal/agent/module", agentModule, "internal/dashboard/report", dashboardReport); violation != "" {
		t.Fatalf("agent module -> dashboard report should be allowed, got %q", violation)
	}
}

func TestApplicationImportsProductCapabilitiesOnlyThroughModules(t *testing.T) {
	// Project is intentionally compile-time-first and has no synthetic runtime
	// module. Its generated HTTP adapter is therefore a valid composition edge.
	compositionAdapters := map[string]bool{
		"internal/project/http": true,
	}
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/app" {
			continue
		}
		for _, imported := range file.imports {
			if !strings.HasPrefix(imported, modulePath+"/internal/") {
				continue
			}
			packagePath := strings.TrimPrefix(imported, modulePath+"/")
			target, ok := ClassifyPackage(packagePath)
			if !ok || target.Capability == "platform" || target.Capability == "composition" {
				continue
			}
			if target.Layer != LayerModule && !compositionAdapters[packagePath] {
				t.Errorf("%s imports product package %s instead of its module surface", file.path, packagePath)
			}
		}
	}
}

func TestDashboardCatalogHasOnlyExplicitProjectionConsumers(t *testing.T) {
	allowed := map[string]bool{
		"internal/project/artifact/artifact.go": true,
		"internal/workspace/module/module.go":   true,
	}
	for _, file := range productionGoFiles(t) {
		if file.pkgDir == "internal/dashboard" || strings.HasPrefix(file.pkgDir, "internal/dashboard/") {
			continue
		}
		for _, imported := range file.imports {
			if imported != modulePath+"/internal/dashboard/catalog" {
				continue
			}
			path := strings.TrimPrefix(file.path, repoRoot(t)+"/")
			if !allowed[path] {
				t.Errorf("%s imports dashboard catalog instead of an owner-specific projection", file.path)
			}
		}
	}
}

func TestAgentOwnsChatUI(t *testing.T) {
	const sharedUI = modulePath + "/internal/workspace/ui"
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/agent" && !strings.HasPrefix(file.pkgDir, "internal/agent/") {
			continue
		}
		for _, imported := range file.imports {
			if imported != sharedUI && !strings.HasPrefix(imported, sharedUI+"/") {
				continue
			}
			t.Errorf("%s imports workspace-owned UI instead of the agent-owned UI contract", file.path)
		}
	}
}

func TestRefreshDoesNotImportWorkspaceUI(t *testing.T) {
	const sharedUI = modulePath + "/internal/workspace/ui"
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/refresh" && !strings.HasPrefix(file.pkgDir, "internal/refresh/") {
			continue
		}
		for _, imported := range file.imports {
			if imported == sharedUI || strings.HasPrefix(imported, sharedUI+"/") {
				t.Errorf("%s imports workspace-owned UI instead of a refresh-owned presentation contract", file.path)
			}
		}
	}
}

func TestDashboardDatastarOwnsItsSignalProjection(t *testing.T) {
	const sharedSignals = modulePath + "/internal/workspace/ui/signals"
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/dashboard/datastar" {
			continue
		}
		for _, imported := range file.imports {
			if imported == sharedSignals {
				t.Errorf("%s imports workspace-owned signals instead of a dashboard-owned projection", file.path)
			}
		}
	}
}

func TestDashboardDoesNotImportWorkspaceUI(t *testing.T) {
	const workspaceUI = modulePath + "/internal/workspace/ui"
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/dashboard" && !strings.HasPrefix(file.pkgDir, "internal/dashboard/") {
			continue
		}
		for _, imported := range file.imports {
			if imported == workspaceUI || strings.HasPrefix(imported, workspaceUI+"/") {
				t.Errorf("%s imports workspace-owned UI", file.path)
			}
		}
	}
}

func TestAdminDoesNotImportWorkspaceUI(t *testing.T) {
	const workspaceUI = modulePath + "/internal/workspace/ui"
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/admin" && !strings.HasPrefix(file.pkgDir, "internal/admin/") {
			continue
		}
		for _, imported := range file.imports {
			if imported == workspaceUI || strings.HasPrefix(imported, workspaceUI+"/") {
				t.Errorf("%s imports workspace-owned UI", file.path)
			}
		}
	}
}

func TestApplicationOwnsGlobalShellComposition(t *testing.T) {
	root := repoRoot(t)
	if !packageDirExists(root, "internal/app/shell") {
		t.Fatal("application shell composition package is missing")
	}
	for _, file := range productionGoFiles(t) {
		if !strings.HasSuffix(file.pkgDir, "/ui") {
			continue
		}
		for _, productNavigation := range []string{
			`ID: "dashboards", Label: "Dashboards"`,
			`ID: "chat", Label: "Chats"`,
			`ID: "workspaces", Label: "Workspaces"`,
			`ID: "admin", Label: "Admin"`,
		} {
			if strings.Contains(file.body, productNavigation) {
				t.Errorf("%s assembles global application navigation %q", file.path, productNavigation)
			}
		}
	}
}

func TestCapabilityRenderersUsePlatformPageMechanism(t *testing.T) {
	const platformPage = modulePath + "/internal/platform/web/page"
	for _, capability := range []string{"access", "admin", "agent", "dashboard", "workspace"} {
		found := false
		for _, file := range productionGoFiles(t) {
			if file.pkgDir != "internal/"+capability+"/ui" {
				continue
			}
			for _, imported := range file.imports {
				if imported == platformPage {
					found = true
				}
			}
			for _, duplicatedHelper := range []string{"func inspectorScript(", "func inspectorElement(", "func pageHead("} {
				if strings.Contains(file.body, duplicatedHelper) {
					t.Errorf("%s retains duplicated document helper %q", file.path, duplicatedHelper)
				}
			}
		}
		if !found {
			t.Errorf("internal/%s/ui does not consume the platform page mechanism", capability)
		}
	}
}

func TestWorkspaceUIContainsNoCapabilityCompatibilitySurface(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/workspace/ui" && !strings.HasPrefix(file.pkgDir, "internal/workspace/ui/") {
			continue
		}
		for _, imported := range file.imports {
			if imported == modulePath+"/internal/admin" ||
				strings.HasPrefix(imported, modulePath+"/internal/admin/") ||
				imported == modulePath+"/internal/agent" ||
				strings.HasPrefix(imported, modulePath+"/internal/agent/") {
				t.Errorf("%s retains another capability's UI compatibility surface through %s", file.path, imported)
			}
		}
	}
}

func TestCapabilitiesDoNotImportWorkspaceSignalContracts(t *testing.T) {
	const sharedSignals = modulePath + "/internal/workspace/ui/signals"
	for _, file := range productionGoFiles(t) {
		owner, ok := ClassifyPackage(file.pkgDir)
		if !ok || owner.Capability == "workspace" || owner.Capability == "composition" || owner.Capability == "ui" {
			continue
		}
		for _, imported := range file.imports {
			if imported == sharedSignals || strings.HasPrefix(imported, sharedSignals+"/") {
				t.Errorf("%s imports workspace-owned signal contracts", file.path)
			}
		}
	}
}

func TestAdminStorageDoesNotImportWorkspaceUI(t *testing.T) {
	const sharedUI = modulePath + "/internal/workspace/ui"
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/admin/storage" {
			continue
		}
		for _, imported := range file.imports {
			if imported == sharedUI || strings.HasPrefix(imported, sharedUI+"/") {
				t.Errorf("%s imports workspace-owned UI instead of admin storage models", file.path)
			}
		}
	}
}

func TestApplicationDoesNotReclaimAccessOrAnalyticsConstruction(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/app" {
			continue
		}
		for _, forbidden := range []string{"analyticsducklake.Open(", "accesssqlite.NewRepository("} {
			if strings.Contains(file.body, forbidden) {
				t.Errorf("%s constructs a migrated capability adapter via %s", file.path, forbidden)
			}
		}
		if strings.HasSuffix(file.path, "/auth.go") {
			t.Errorf("%s owns authentication behavior; move it to access/module", file.path)
		}
	}
}

func TestAppDoesNotRetainPlatformStore(t *testing.T) {
	root := repoRoot(t)
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/app" {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, filepath.FromSlash(file.path)), nil, 0)
		require.NoError(t, err)
		ast.Inspect(parsed, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Store" {
				return true
			}
			if ident, ok := selector.X.(*ast.Ident); ok && ident.Name == "platform" {
				t.Errorf("%s retains platform.Store; keep the store local to application assembly", file.path)
			}
			return true
		})
	}
}

func TestOnlyCompositionImportsApplicationPackage(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		for _, imported := range file.imports {
			if imported != modulePath+"/internal/app" {
				continue
			}
			rule, ok := ClassifyPackage(file.pkgDir)
			if !ok || rule.Layer != LayerComposition {
				t.Errorf("%s imports internal/app outside process composition", file.path)
			}
		}
	}
}

func TestLegacyApplicationContainerAPIIsAbsent(t *testing.T) {
	root := repoRoot(t)
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/app" {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, filepath.FromSlash(file.path)), nil, 0)
		require.NoError(t, err)
		for _, declaration := range parsed.Decls {
			switch value := declaration.(type) {
			case *ast.GenDecl:
				for _, specification := range value.Specs {
					if named, ok := specification.(*ast.TypeSpec); ok {
						switch named.Name.Name {
						case "Server", "server", "Options", "serverOptions", "Host", "host":
							t.Errorf("%s declares legacy application container type %s", file.path, named.Name.Name)
						}
					}
				}
			case *ast.FuncDecl:
				if value.Recv == nil {
					switch value.Name.Name {
					case "New", "NewWithOptions", "newServer", "newServerWithOptions", "buildServer":
						t.Errorf("%s declares legacy application constructor %s", file.path, value.Name.Name)
					}
				}
			}
		}
	}
}

func TestRequestRuntimeDoesNotRetainConstructionDependencies(t *testing.T) {
	root := repoRoot(t)
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/app" {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, filepath.FromSlash(file.path)), nil, 0)
		require.NoError(t, err)
		for _, declaration := range parsed.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, specification := range generic.Specs {
				named, ok := specification.(*ast.TypeSpec)
				if !ok || named.Name.Name != "runtimeRouter" {
					continue
				}
				structure, ok := named.Type.(*ast.StructType)
				if !ok {
					t.Fatalf("%s runtimeRouter must be a struct", file.path)
				}
				for _, field := range structure.Fields.List {
					for _, name := range field.Names {
						switch name.Name {
						case "adminDatabase", "servingStateRepo", "managedDataResolver",
							"workspaceRepo", "workspacePersistence", "workspaceAssetCatalog",
							"accessRepo", "reloader", "duckLakeCatalogPath", "duckLakeDataPath",
							"jobLeaseTimeout", "deploymentConfig":
							t.Errorf("%s runtimeRouter retains construction dependency %s", file.path, name.Name)
						}
					}
				}
			}
		}
	}
}

func TestAppDoesNotConstructRepositoriesFromSQLDB(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		if file.pkgDir == "internal/app" && file.path != "internal/app/composition.go" && strings.Contains(file.body, ".SQLDB()") {
			t.Errorf("%s constructs adapters from platform.Store; capability modules must receive construction ownership", file.path)
		}
	}
}

func TestWorkloadImportsNoProductCapabilities(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		if file.pkgDir != "internal/workload" {
			continue
		}
		for _, imported := range file.imports {
			if strings.HasPrefix(imported, modulePath+"/internal/") {
				t.Fatalf("%s imports product capability %s", file.path, imported)
			}
		}
	}
}

func TestOnlyWorkloadAdaptersAndCompositionDependOnWorkload(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		for _, imported := range file.imports {
			if imported != modulePath+"/internal/workload" {
				continue
			}
			if !AllowsWorkloadImport(file.pkgDir) {
				t.Fatalf("%s depends on workload outside composition or an execution/worker adapter", file.path)
			}
		}
	}
}

func TestArrowImportsStayInsideAnalyticalDataPlaneAndExplicitEncoders(t *testing.T) {
	allowed := []string{
		"internal/analytics/arrowquery",
		"internal/analytics/arrowresult",
		"internal/analytics/resultcache",
		"internal/analytics/materialize",
		"internal/analytics/ducklake",
		"internal/dashboard/semanticapi",
		"internal/dashboard/http",
	}
	for _, file := range productionGoFiles(t) {
		for _, imported := range file.imports {
			if !strings.HasPrefix(imported, "github.com/apache/arrow-go/") {
				continue
			}
			permitted := false
			for _, prefix := range allowed {
				if file.pkgDir == prefix || strings.HasPrefix(file.pkgDir, prefix+"/") {
					permitted = true
					break
				}
			}
			if !permitted {
				t.Fatalf("%s imports Arrow outside the analytical data plane or an explicit Arrow encoder", file.path)
			}
		}
	}
}

func TestUseCasesDoNotImportAdapters(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		if !isInternalPackage(file.pkgDir) || isAdapterOrCompositionPackage(file.pkgDir) {
			continue
		}
		for _, imported := range file.imports {
			if isForbiddenUseCaseImport(imported) {
				t.Fatalf("%s imports adapter or transport package %s", file.path, imported)
			}
		}
	}
}

func TestCapabilityAPIPackagesAreTransportContractOnly(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		if !strings.HasSuffix(file.pkgDir, "/api") || file.pkgDir == "internal/app/api" {
			continue
		}
		for _, imported := range file.imports {
			if imported == modulePath+"/internal/app" ||
				imported == "net/http" ||
				imported == "github.com/go-chi/chi/v5" ||
				strings.Contains(imported, "datastar") ||
				strings.Contains(imported, "gomponents") {
				t.Fatalf("%s imports forbidden API dependency %s", file.path, imported)
			}
		}
	}
}

func TestCapabilityUIPackagesAreRenderOnly(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		if !strings.HasSuffix(file.pkgDir, "/ui") {
			continue
		}
		for _, imported := range file.imports {
			if imported == modulePath+"/internal/app/api/gen" ||
				imported == modulePath+"/internal/platform/db" ||
				imported == "database/sql" ||
				imported == "net/http" ||
				imported == "github.com/go-chi/chi/v5" {
				t.Fatalf("%s imports forbidden UI dependency %s", file.path, imported)
			}
		}
		for _, forbidden := range []string{".QueryContext(", ".QueryRowContext(", ".ExecContext("} {
			if strings.Contains(file.body, forbidden) {
				t.Fatalf("%s performs storage access via %s", file.path, forbidden)
			}
		}
	}
}

func TestStaticSQLiteAdaptersUseGeneratedQueries(t *testing.T) {
	generatedOnly := map[string]bool{
		"internal/agent/sqlite":                 true,
		"internal/dashboard/publication/sqlite": true,
		"internal/dashboard/session/sqlite":     true,
		"internal/deployment/sqlite":            true,
		"internal/manageddata/sqlite":           true,
		"internal/servingstate/sqlite":          true,
		"internal/workspace/sqlite":             true,
	}
	generatedOnlyFiles := map[string]bool{
		"internal/access/sqlite/api_symmetry.go":             true,
		"internal/access/sqlite/authorization.go":            true,
		"internal/refresh/sqlite/runs.go":                    true,
		"internal/analytics/queryaudit/sqlite/repository.go": true,
	}
	for _, file := range productionGoFiles(t) {
		if !generatedOnly[file.pkgDir] && !generatedOnlyFiles[file.path] {
			continue
		}
		for _, directCall := range []string{".QueryContext(", ".QueryRowContext(", ".ExecContext("} {
			if strings.Contains(file.body, directCall) {
				t.Fatalf("%s bypasses sqlc via %s", file.path, directCall)
			}
		}
	}
}

func TestCapabilitySQLiteAdaptersDoNotImportOtherSQLiteAdapters(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		if !strings.Contains(file.pkgDir, "/sqlite") {
			continue
		}
		for _, imported := range file.imports {
			if !strings.HasPrefix(imported, modulePath+"/internal/") || !strings.Contains(imported, "/sqlite") {
				continue
			}
			packagePath := strings.TrimPrefix(imported, modulePath+"/")
			source, sourceOK := ClassifyPackage(file.pkgDir)
			target, targetOK := ClassifyPackage(packagePath)
			if sourceOK && targetOK && source.Capability == target.Capability {
				continue
			}
			t.Errorf("%s imports persistence implementation %s; use a consumer-owned port or module bridge", file.path, imported)
		}
	}
}

func TestDashboardPersistenceDoesNotWriteAccessTables(t *testing.T) {
	root := repoRoot(t)
	dashboardRoot := filepath.Join(root, "internal", "dashboard")
	err := filepath.WalkDir(dashboardRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		extension := filepath.Ext(path)
		if entry.IsDir() || !strings.Contains(filepath.ToSlash(path), "/sqlite/") ||
			(extension != ".go" && extension != ".sql") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for _, statement := range []string{
			"INSERT INTO principals",
			"UPDATE principals",
			"DELETE FROM principals",
		} {
			if strings.Contains(string(body), statement) {
				t.Errorf("%s writes the Access-owned principals table via %q; use an Access-owned operation", filepath.ToSlash(relative), statement)
			}
		}
		return nil
	})
	require.NoError(t, err)
}

func TestGeneratedPlatformQueriesStayInsidePlatform(t *testing.T) {
	const sharedQueries = modulePath + "/internal/platform/db"
	for _, file := range productionGoFiles(t) {
		if file.pkgDir == "internal/platform" || strings.HasPrefix(file.pkgDir, "internal/platform/db") {
			continue
		}
		for _, imported := range file.imports {
			if imported == sharedQueries {
				t.Errorf("%s imports the shared generated query package; generate capability-private queries instead", file.path)
			}
		}
	}
}

func TestPlatformSQLCOmitsUnusedCapabilityModels(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "sqlc.yaml"))
	require.NoError(t, err)
	config := string(body)
	start := strings.Index(config, `queries: "internal/platform/db/queries"`)
	end := strings.Index(config, `"internal/analytics/queryaudit/sqlite/queries"`)
	if start < 0 || end < 0 || end <= start {
		t.Fatal("platform sqlc generation block is missing")
	}
	if !strings.Contains(config[start:end], "omit_unused_structs: true") {
		t.Fatal("platform sqlc generation exposes unused product-capability models")
	}
}

func TestFixedOperationalRetentionQueriesUseSQLC(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		if file.path != "internal/admin/sqlite/retention.go" {
			continue
		}
		if strings.Contains(file.body, "DELETE FROM api_async_events") {
			t.Fatalf("%s embeds the fixed async-event retention query instead of using sqlc", file.path)
		}
	}
}

func TestSQLCQueriesAreSplitByDomain(t *testing.T) {
	root := repoRoot(t)
	for _, domain := range []string{
		"internal/admin/sqlite/queries/retention.sql",
		"internal/access/sqlite/queries/access.sql",
		"internal/agent/sqlite/queries/agent.sql",
		"internal/platform/http/idempotency/sqlite/queries/idempotency.sql",
		"internal/platform/http/cursorsigning/sqlite/queries/cursor_signing.sql",
		"internal/deployment/sqlite/queries/deployment.sql",
		"internal/dashboard/publication/sqlite/queries/publication.sql",
		"internal/dashboard/session/sqlite/queries/session.sql",
		"internal/manageddata/sqlite/queries/managed_data.sql",
		"internal/refresh/sqlite/runqueries/materialization.sql",
		"internal/platform/jobs/sqlite/queries/async_job.sql",
		"internal/platform/db/queries/platform.sql",
		"internal/analytics/queryaudit/sqlite/queries/query_history.sql",
		"internal/refresh/sqlite/schedulequeries/refresh_pipeline.sql",
		"internal/release/sqlite/queries/release.sql",
		"internal/servingstate/sqlite/queries/serving_state.sql",
		"internal/workspace/sqlite/queries/workspace.sql",
	} {
		contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(domain)))
		if err != nil {
			t.Fatalf("read sqlc query domain %s: %v", domain, err)
		}
		if !strings.Contains(string(contents), "-- name:") {
			t.Fatalf("sqlc query domain %s contains no named queries", domain)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "platform", "db", "queries.sql")); !os.IsNotExist(err) {
		t.Fatal("legacy sqlc query monolith must not exist")
	}
}

func TestSQLCUsesRuntimeMigrationsAsItsSchemaSource(t *testing.T) {
	root := repoRoot(t)
	config, err := os.ReadFile(filepath.Join(root, "sqlc.yaml"))
	if err != nil {
		t.Fatalf("read sqlc config: %v", err)
	}
	if !strings.Contains(string(config), `schema: "internal/platform/migrations"`) {
		t.Fatal("sqlc must compile against the runtime Goose migrations")
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "platform", "db", "schema.sql")); !os.IsNotExist(err) {
		t.Fatal("duplicate sqlc schema snapshot must not exist")
	}
}

func TestRequiredCapabilityAdaptersExist(t *testing.T) {
	root := repoRoot(t)
	for _, dir := range []string{
		"internal/access/http",
		"internal/admin/cli",
		"internal/admin/http",
		"internal/agent/cli",
		"internal/agent/http",
		"internal/analytics/connectors",
		"internal/refresh/http",
		"internal/dashboard/cli",
		"internal/dashboard/semanticapi",
		"internal/dashboard/http",
		"internal/manageddata/cli",
		"internal/project/cli",
		"internal/workspace/datastar",
		"internal/workspace/cli",
		"internal/workspace/http",
	} {
		if !packageDirExists(root, dir) {
			t.Fatalf("required capability adapter package %s does not exist", dir)
		}
	}
}

func TestPlatformStoreSQLDBDoesNotLeakPastCompositionAndAdapters(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		if !strings.Contains(file.body, ".SQLDB()") {
			continue
		}
		if isSQLDBAllowedFile(file) {
			continue
		}
		t.Fatalf("%s calls platform Store SQLDB outside composition or adapter construction", file.path)
	}
}

func TestRemovedLegacyAgentPackagesAreNotImported(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		for _, imported := range file.imports {
			switch imported {
			case modulePath + "/internal/agentapp",
				modulePath + "/internal/agentapp/sqlite",
				modulePath + "/internal/agenttools",
				modulePath + "/internal/agentconfig":
				t.Fatalf("%s imports legacy agent package %s", file.path, imported)
			}
		}
	}
}

func TestSecretComparisonsGoThroughSecretPackage(t *testing.T) {
	for _, file := range productionGoFiles(t) {
		if file.pkgDir == "internal/platform/security/secret" {
			continue
		}
		for _, imported := range file.imports {
			if imported == "crypto/subtle" {
				t.Fatalf("%s imports crypto/subtle directly; use internal/platform/security/secret for fixed-size secret comparisons", file.path)
			}
		}
	}
}

func TestProductionContainerContractExists(t *testing.T) {
	root := repoRoot(t)
	dockerfile, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	text := string(dockerfile)
	for _, want := range []string{
		"FROM node:24-bookworm@sha256:",
		"FROM golang:1.25-bookworm@sha256:",
		"AS go-deps",
		"FROM go-deps AS sourcegen",
		"COPY --from=node /usr/local/bin/node /usr/local/bin/node",
		"COPY --from=node /usr/local/lib/node_modules /usr/local/lib/node_modules",
		"ln -sf ../lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm",
		"./scripts/generate_build_sources.sh",
		"go run ./internal/app/tools/mapassets --out .data/map-assets",
		"go run ./internal/app/tools/clidocgen",
		"go run ./internal/app/tools/schemadocgen",
		"go run ./internal/app/tools/openapidocgen",
		"go run ./internal/app/tools/docsitegen",
		"FROM oven/bun:1.3.7@sha256:",
		"COPY --from=sourcegen /src/api/gen ./api/gen",
		"COPY --from=sourcegen /src/api/visualization ./api/visualization",
		"COPY --from=sourcegen /src/web/generated ./web/generated",
		"RUN bun install --frozen-lockfile --no-cache",
		"bun scripts/generate_visualization_validator.ts",
		"bun run build",
		"FROM go-deps AS build",
		"COPY --from=sourcegen /src/internal/access/api/gen ./internal/access/api/gen",
		"COPY --from=sourcegen /src/internal/agent/api/gen ./internal/agent/api/gen",
		"COPY --from=sourcegen /src/internal/analytics/api/gen ./internal/analytics/api/gen",
		"COPY --from=sourcegen /src/internal/dashboard/api/gen ./internal/dashboard/api/gen",
		"COPY --from=sourcegen /src/internal/app/api/aggregate ./internal/app/api/aggregate",
		"COPY --from=sourcegen /src/internal/app/api/gen ./internal/app/api/gen",
		"COPY --from=sourcegen /src/internal/deployment/api/gen ./internal/deployment/api/gen",
		"COPY --from=sourcegen /src/internal/manageddata/api/gen ./internal/manageddata/api/gen",
		"COPY --from=sourcegen /src/internal/platform/http/api/gen ./internal/platform/http/api/gen",
		"COPY --from=sourcegen /src/internal/project/api/gen ./internal/project/api/gen",
		"COPY --from=sourcegen /src/internal/refresh/api/gen ./internal/refresh/api/gen",
		"COPY --from=sourcegen /src/internal/release/api/gen ./internal/release/api/gen",
		"COPY --from=sourcegen /src/internal/workspace/api/gen ./internal/workspace/api/gen",
		"COPY --from=sourcegen /src/internal/access/ui/signals/models.gen.go ./internal/access/ui/signals/models.gen.go",
		"COPY --from=sourcegen /src/internal/admin/ui/signals/models.gen.go ./internal/admin/ui/signals/models.gen.go",
		"COPY --from=sourcegen /src/internal/agent/ui/signals/models.gen.go ./internal/agent/ui/signals/models.gen.go",
		"COPY --from=sourcegen /src/internal/dashboard/ui/signals/models.gen.go ./internal/dashboard/ui/signals/models.gen.go",
		"COPY --from=sourcegen /src/internal/workspace/ui/signals/models.gen.go ./internal/workspace/ui/signals/models.gen.go",
		"COPY --from=sourcegen /src/docs ./docs",
		"CGO_ENABLED=1 go build",
		"FROM debian:bookworm-slim@sha256:",
		"USER leapview",
		"WORKDIR /app",
		"COPY --from=web /src/static ./static",
		"COPY --from=sourcegen /src/.data/map-assets ./.data/map-assets",
		"LEAPVIEW_HOME=/var/lib/leapview/home",
		"LEAPVIEW_MANAGED_DATA_DIR=/var/lib/leapview/home/managed-data",
		"LEAPVIEW_PRODUCTION=1",
		"HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD [\"leapview\", \"healthcheck\"]",
		"CMD [\"serve\", \"--production\"]",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Dockerfile missing production container contract fragment %q", want)
		}
	}
	if count := strings.Count(text, "RUN go mod download"); count != 1 {
		t.Fatalf("Dockerfile downloads Go modules %d times, want one shared dependency stage", count)
	}
	const seededModuleCache = "type=cache,id=leapview-go-mod,target=/go/pkg/mod,from=go-deps,source=/go/pkg/mod,sharing=locked"
	if count := strings.Count(text, seededModuleCache); count != 3 {
		t.Fatalf("Dockerfile uses the seeded persistent Go module cache %d times, want source generation, map extraction, and compilation", count)
	}

	ignored, err := os.ReadFile(filepath.Join(root, ".dockerignore"))
	if err != nil {
		t.Fatalf("read .dockerignore: %v", err)
	}
	ignoreText := string(ignored)
	for _, want := range []string{
		".data", ".leapview", "node_modules", "**/.tmp", "api/gen", "internal/access/api/gen", "internal/agent/api/gen", "internal/analytics/api/gen", "internal/dashboard/api/gen", "internal/deployment/api/gen", "internal/manageddata/api/gen",
		"internal/app/api/aggregate", "internal/app/api/gen", "internal/platform/http/api/gen", "internal/project/api/gen", "internal/refresh/api/gen", "internal/release/api/gen", "internal/workspace/api/gen", "static/chunks",
	} {
		if !strings.Contains(ignoreText, want) {
			t.Fatalf(".dockerignore missing generated or runtime path %q", want)
		}
	}
}

func TestGeographicRendererDecisionIsExplicitAndNavigable(t *testing.T) {
	root := repoRoot(t)
	decision, err := os.ReadFile(filepath.Join(root, "docs", "articles", "architecture", "geographic-rendering.md"))
	if err != nil {
		t.Fatalf("read geographic rendering decision: %v", err)
	}
	text := string(decision)
	for _, want := range []string{
		"# Geographic rendering decision",
		"Status: accepted",
		"MapLibre is the sole geographic renderer",
		"ECharts `geo`",
		"one geographic camera",
		"same-origin",
		"spatial-windowed",
		"| Capability | MapLibre | ECharts `geo` |",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("geographic rendering decision missing %q", want)
		}
	}
	navigation, err := os.ReadFile(filepath.Join(root, "docs", "navigation.yaml"))
	if err != nil {
		t.Fatalf("read docs navigation: %v", err)
	}
	if !strings.Contains(string(navigation), "source: articles/architecture/geographic-rendering.md") {
		t.Fatal("geographic rendering decision is not registered in documentation navigation")
	}
}

func TestPublicSiteProductionContainerContractExists(t *testing.T) {
	root := repoRoot(t)
	dockerfile, err := os.ReadFile(filepath.Join(root, "Dockerfile.site"))
	if err != nil {
		t.Fatalf("read Dockerfile.site: %v", err)
	}
	text := string(dockerfile)
	for _, want := range []string{
		"FROM node:24-bookworm@sha256:",
		"FROM golang:1.25-bookworm@sha256:",
		"./scripts/generate_build_sources.sh",
		"go run -tags=duckdb_arrow ./internal/app/tools/visualdocgen",
		"FROM oven/bun:1.3.7@sha256:",
		"COPY --from=sourcegen /src/api/gen ./api/gen",
		"COPY --from=sourcegen /src/api/visualization ./api/visualization",
		"COPY --from=sourcegen /src/web/generated ./web/generated",
		"RUN bun install --frozen-lockfile --no-cache",
		"bun scripts/generate_visualization_validator.ts",
		"bun run build:site",
		"FROM golang:1.25-bookworm@sha256:",
		"CGO_ENABLED=0 go build -trimpath",
		"./cmd/leapview-site",
		"FROM gcr.io/distroless/static-debian12:nonroot@sha256:",
		"USER nonroot:nonroot",
		"ENV LEAPVIEW_SITE_BASE_URL=",
		"ENTRYPOINT [\"/leapview-site\"]",
		"CMD [\"-addr=:8081\"]",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("Dockerfile.site missing production container contract fragment %q", want)
		}
	}
	if strings.Contains(text, "apigen@v0.4.0") || strings.Contains(text, "apigenpostprocess") {
		t.Error("Dockerfile.site still uses the retired APIGen v0.4 generation pipeline")
	}
}

func TestBuildSourceGenerationContract(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "scripts", "generate_build_sources.sh")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat shared build source generator: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatal("shared build source generator is not executable")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shared build source generator: %v", err)
	}
	text := string(body)
	commands := []string{
		"go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate",
		"go run ./internal/app/tools/configgen",
		"go run ./internal/app/tools/layoutcontractgen",
		"typespec-compile -manifest api/apigen.yaml -target leapview-v1",
		"typespec-compile -manifest api/apigen.yaml -target ui-signals",
		"typespec-compile -manifest api/apigen.yaml -target visualization-ir",
		"all -manifest api/apigen.yaml -target visualization-ir",
		"schema export --format json-schema --out schemas/json",
	}
	previous := -1
	for _, command := range commands {
		current := strings.Index(text, command)
		if current < 0 {
			t.Fatalf("shared build source generator missing command %q", command)
		}
		if current <= previous {
			t.Fatalf("shared build source generator command %q is out of order", command)
		}
		previous = current
	}
}

func TestResponsiveLayoutContractGenerationIsAvailableToEveryBrowserBuild(t *testing.T) {
	root := repoRoot(t)
	files := map[string][]string{
		"Taskfile.yml": {
			"layout-contract:generate:",
			"internal/project/layoutcontract/contracts.json",
			"web/generated/dashboard-layout/contracts.json",
			"go run ./internal/app/tools/layoutcontractgen",
			"build:\n    desc: Build browser assets\n    deps:\n      - node:deps\n      - layout-contract:generate",
			"site:build:\n    desc: Build the LeapView public site assets from generated contracts",
			"- task: layout-contract:generate",
		},
		filepath.Join("scripts", "generate_build_sources.sh"): {
			"go run ./internal/app/tools/layoutcontractgen",
		},
		filepath.Join("web", "components", "dashboard", "visualization", "layout.ts"): {
			"../../../generated/dashboard-layout/contracts.json",
		},
	}
	for name, fragments := range files {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, fragment := range fragments {
			if !strings.Contains(string(body), fragment) {
				t.Errorf("%s missing responsive layout generation fragment %q", name, fragment)
			}
		}
	}
}

func TestCoreProceduralGuidesUseTheOperationalTemplate(t *testing.T) {
	root := repoRoot(t)
	guides := []string{
		"docs/articles/start/installation.md",
		"docs/articles/start/first-dashboard.md",
		"docs/articles/build/connect-data.md",
		"docs/articles/build/model-tables.md",
		"docs/articles/build/semantic-model.md",
		"docs/articles/build/dashboard.md",
		"docs/guides/cli/validate-deploy.md",
		"docs/articles/operate/self-hosting.md",
		"docs/articles/security/oidc.md",
		"docs/articles/integrate/api-quickstart.md",
	}
	for _, guide := range guides {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(guide)))
		if err != nil {
			t.Errorf("read %s: %v", guide, err)
			continue
		}
		text := string(body)
		for _, section := range []string{"\n## Before you begin\n", "\n## Validate", "\n## Verify", "\n## Troubleshooting\n", "\n## Next steps\n"} {
			if !strings.Contains(text, section) {
				t.Errorf("%s missing procedural section %q", guide, strings.TrimSpace(section))
			}
		}
		if !strings.Contains(text, "\n1. ") {
			t.Errorf("%s does not contain a numbered procedure", guide)
		}
	}
}

func TestDevelopmentServerTracksCompiledFallbackProcess(t *testing.T) {
	root := repoRoot(t)
	server, err := os.ReadFile(filepath.Join(root, "scripts", "dev-server.sh"))
	if err != nil {
		t.Fatalf("read development server script: %v", err)
	}
	serverText := string(server)
	for _, want := range []string{
		`go build -tags=duckdb_arrow -o "$TMP_DIR/leapview-dev" ./cmd/leapview`,
		`"$TMP_DIR/leapview-dev" >> "$LOG_FILE" 2>&1 &`,
		`LEAPVIEW_MANAGED_DATA_MIN_FREE_BYTES="${LEAPVIEW_MANAGED_DATA_MIN_FREE_BYTES:-67108864}"`,
	} {
		if !strings.Contains(serverText, want) {
			t.Fatalf("development server script missing tracked binary fragment %q", want)
		}
	}
	if strings.Contains(serverText, `go run ./cmd/leapview >> "$LOG_FILE" 2>&1 &`) {
		t.Fatal("development server must not track the go run wrapper as the server process")
	}

	qa, err := os.ReadFile(filepath.Join(root, "scripts", "qa_ui_framework.ts"))
	if err != nil {
		t.Fatalf("read UI framework QA script: %v", err)
	}
	qaText := string(qa)
	if !strings.Contains(qaText, "const managedServerReadyAttempts = 1800") ||
		!strings.Contains(qaText, "attempt < managedServerReadyAttempts") {
		t.Fatal("UI framework QA must allow a cold Go build before checking server readiness")
	}
	for _, want := range []string{
		"LEAPVIEW_MANAGED_DATA_DIR: `${qaHome}/managed-data`",
		"['chmod', '-R', 'u+w', qaHome]",
	} {
		if !strings.Contains(qaText, want) {
			t.Fatalf("UI framework QA must isolate and clean managed-data state: missing %q", want)
		}
	}
}

func TestContinuousIntegrationWorkflowRunsProductionGates(t *testing.T) {
	root := repoRoot(t)
	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("read CI workflow: %v", err)
	}
	taskfile, err := os.ReadFile(filepath.Join(root, "Taskfile.yml"))
	if err != nil {
		t.Fatalf("read Taskfile.yml: %v", err)
	}
	text := string(workflow)
	for _, want := range []string{
		"name: CI",
		"pull_request:",
		"push:",
		"workflow_dispatch:",
		"autback-ci:",
		"name: Autback CI",
		"environment: autback",
		"id-token: write",
		"packages: write",
		"uses: flidai/autback/action/setup-autback@e3e9668cc4e5d81a2204fa014bb9de228fa510d0",
		"version: 0.1.6",
		"service-url: ${{ vars.AUTBACK_SERVICE_URL }}",
		"project: leapview",
		"ca-certificate: ${{ vars.AUTBACK_CA_CERTIFICATE }}",
		"autback doctor",
		"--file Dockerfile.autback",
		"autback build --",
		"--platform linux/amd64",
		"--push",
		"--metadata-file",
		"containerimage.digest",
		"autback exec --image \"${AUTBACK_RUNNER_IMAGE}\" --timeout 90m",
		"-- task ci:local",
		"--cache go-build=/root/.cache/go-build",
		"--cache go-mod=/go/pkg/mod",
		"--cache bun=/root/.bun/install/cache",
		"--cache terraform=/root/.cache/terraform",
		"--file Dockerfile",
		"task image:qualify:production IMAGE=\"${immutable_image}\"",
		"--file Dockerfile.site",
		"task image:qualify:site IMAGE=\"${immutable_image}\"",
		"github-ci:",
		"name: GitHub CI (external pull request)",
		"github.event.pull_request.head.repo.full_name != github.repository",
		"github.actor == 'dependabot[bot]'",
		"actions/setup-go@",
		"go-version-file: go.mod",
		"oven-sh/setup-bun@",
		"bun-version: 1.3.7",
		"go install github.com/go-task/task/v3/cmd/task@v3.50.0",
		"run: task ci:local",
		"ci-gate:",
		"name: CI gate",
		"needs: [autback-ci, github-ci]",
		"success:skipped|skipped:success",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("CI workflow missing production gate fragment %q", want)
		}
	}
	autbackCI := workflowJobBlock(t, text, "autback-ci")
	githubCI := workflowJobBlock(t, text, "github-ci")
	for _, forbidden := range []string{
		"allow-source-fallback",
		"autback-poc",
		"depot/",
		"type=gha",
		"--load",
		"--cpus",
		"--memory",
		"actions/upload-artifact@",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("CI workflow retains superseded runner fragment %q", forbidden)
		}
	}
	if strings.Contains(githubCI, "id-token: write") || strings.Contains(githubCI, "setup-autback") {
		t.Fatal("untrusted pull requests must not receive Autback OIDC access")
	}
	for _, want := range []string{"task ci:local", "Dockerfile.autback", "Dockerfile.site", "task image:qualify:production", "task image:qualify:site"} {
		if !strings.Contains(autbackCI, want) {
			t.Fatalf("trusted Autback CI must own the complete build contract: missing %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "scripts", "autback_exec.sh")); !os.IsNotExist(err) {
		t.Fatalf("Autback execution must use the signal-aware Go CLI directly: %v", err)
	}
	taskText := string(taskfile)
	ciDispatcher := taskfileTaskBlock(t, taskText, "ci")
	for _, want := range []string{"autback exec", "-- task ci:local"} {
		if !strings.Contains(ciDispatcher, want) {
			t.Fatalf("ci must dispatch the canonical workload to Autback: missing %q", want)
		}
	}
	ciLocal := taskfileTaskBlock(t, taskText, "ci:local")
	for _, want := range []string{"- task: generate", "- task: test:go", "- task: generated:check", "go vet ./...", "- task: deploy:check"} {
		if !strings.Contains(ciLocal, want) {
			t.Fatalf("ci:local must own the complete current-machine contract: missing %q", want)
		}
	}
	for _, retired := range []string{"test", "autback:test", "autback:ci"} {
		if strings.Contains(taskText, "  "+retired+":\n") {
			t.Fatalf("Taskfile retains redundant top-level target %q", retired)
		}
	}
	deployCheck := taskfileTaskBlock(t, taskText, "deploy:check")
	if !strings.Contains(deployCheck, "- api:generate") {
		t.Fatal("deploy:check must generate its build-only API inputs")
	}
	siteImageQualification := taskfileTaskBlock(t, taskText, "image:qualify:site")
	if !strings.Contains(siteImageQualification, "- task: api:generate") {
		t.Fatal("site image qualification must generate the leapviewctl API inputs in a clean checkout")
	}
	for _, want := range []string{
		"config:generate:",
		"go run ./internal/app/tools/configgen",
		"config:check:",
		"go run ./internal/app/tools/configgen --check",
		"node:audit:",
		"bun audit",
		"vuln:",
		"golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...",
		"ci:prepare:frontend:",
		"ci:test:docs-site:",
		"go test ./cmd/leapview-site ./docs ./site ./internal/app/site/...",
		"ci:test:frontend:core:",
		"ci:test:frontend:reports:",
		"ci:test:frontend:chat:",
		"ci:test:frontend:workspace:",
		"ci:test:frontend:site:",
		"test:go:",
		"task --parallel test:go:packages test:go:app:0 test:go:app:1 test:go:app:2 test:go:app:3",
		"go list ./... | grep -v '/internal/app$' | xargs go test -p 2",
		"--shard-count 4",
		"image:qualify:production:",
		"TMPDIR={{.ROOT_DIR}}/.tmp/qualification/tmp",
		"go run ./cmd/leapviewctl qualify image",
		"--require-immutable",
		"image:qualify:site:",
		"go run ./cmd/leapviewctl qualify site-image",
	} {
		if !strings.Contains(taskText, want) {
			t.Fatalf("Taskfile missing vulnerability gate fragment %q", want)
		}
	}
	var packageManifest struct {
		Scripts map[string]string `json:"scripts"`
	}
	packageJSON, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		t.Fatalf("read package.json: %v", err)
	}
	if err := json.Unmarshal(packageJSON, &packageManifest); err != nil {
		t.Fatalf("decode package.json: %v", err)
	}
	for script := range packageManifest.Scripts {
		if strings.HasPrefix(script, "test:") && !strings.Contains(taskText, "bun run "+script) {
			t.Errorf("frontend test script %q is not assigned to a Taskfile CI shard", script)
		}
	}
	for _, retired := range []string{
		"scripts/benchmark_autback_digest_push.sh",
		"scripts/qualify_production_image.sh",
		"scripts/smoke_production_image.sh",
		"scripts/smoke_site_image.sh",
	} {
		if _, err := os.Stat(filepath.Join(root, retired)); !os.IsNotExist(err) {
			t.Fatalf("retired Autback shell implementation still exists at %s: %v", retired, err)
		}
	}
}

func TestContinuousIntegrationHealthWorkflowReportsAndAlerts(t *testing.T) {
	root := repoRoot(t)
	workflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci-health.yml"))
	if err != nil {
		t.Fatalf("read CI health workflow: %v", err)
	}
	text := string(workflow)
	for _, want := range []string{
		"name: CI health",
		"schedule:",
		"workflow_dispatch:",
		"actions: read",
		"issues: write",
		"go run ./internal/app/tools/cireport",
		"--days 7",
		"name: ci-health",
		"retention-days: 30",
		"CI health thresholds exceeded",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("CI health workflow missing fragment %q", want)
		}
	}
	for _, forbidden := range []string{"id-token: write", "depot/", "--depot-builds", "depot-builds.json"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("CI health workflow retains retired Depot fragment %q", forbidden)
		}
	}
}

func TestLeapViewDeclaresGenericAutbackConsumerContract(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "depot.json")); !os.IsNotExist(err) {
		t.Fatalf("retired Depot project configuration still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "autback")); !os.IsNotExist(err) {
		t.Fatalf("generic Autback implementation must live in flidai/autback, not LeapView: %v", err)
	}
	link, err := os.ReadFile(filepath.Join(root, "autback.json"))
	if err != nil {
		t.Fatalf("read Autback project link: %v", err)
	}
	if strings.TrimSpace(string(link)) != `{"project":"leapview"}` {
		t.Fatalf("autback.json must contain only the LeapView project link, got %s", link)
	}
	packageJSON, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		t.Fatalf("read package manifest: %v", err)
	}
	if !strings.Contains(string(packageJSON), `"typescript": "5.9.3"`) {
		t.Fatal("LeapView must pin the TypeScript compiler used by its remote test contract")
	}
	runner, err := os.ReadFile(filepath.Join(root, "Dockerfile.autback"))
	if err != nil {
		t.Fatalf("read Autback runner image: %v", err)
	}
	runnerText := string(runner)
	for _, want := range []string{
		"docker:29.1.3-cli@sha256:",
		"golang:1.25-bookworm@sha256:",
		"oven/bun:1.3.7@sha256:",
		"hashicorp/terraform:1.13.5@sha256:",
		"github.com/go-task/task/v3/cmd/task@v3.50.0",
		"github.com/bufbuild/buf/cmd/buf@v1.57.2",
		"@playwright/test@1.61.1",
		"playwright install --with-deps chromium",
		"PLAYWRIGHT_BROWSERS_PATH=/ms-playwright",
		"COPY --from=docker-cli /usr/local/bin/docker /usr/local/bin/docker",
		"COPY --from=docker-cli /usr/local/libexec/docker/cli-plugins /usr/local/libexec/docker/cli-plugins",
	} {
		if !strings.Contains(runnerText, want) {
			t.Fatalf("Dockerfile.autback missing %q", want)
		}
	}
	operationsWorkflow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "autback.yml"))
	if err != nil {
		t.Fatalf("read Autback operations workflow: %v", err)
	}
	operationsText := string(operationsWorkflow)
	for _, want := range []string{
		"name: Autback runner image",
		"publish:",
		"environment: autback",
		"packages: write",
		"docker/login-action@",
		"uses: flidai/autback/action/setup-autback@e3e9668cc4e5d81a2204fa014bb9de228fa510d0",
		"version: 0.1.6",
		"autback image build",
		"--file Dockerfile.autback",
		"--platform linux/amd64",
		"ghcr.io/flidai/leapview:autback-${{ github.sha }}",
	} {
		if !strings.Contains(operationsText, want) {
			t.Fatalf("Autback operations workflow missing runner publication fragment %q", want)
		}
	}
	for _, forbidden := range []string{
		"allow-source-fallback",
		"autback-poc",
		"--cpus",
		"--memory",
		"benchmark_push",
		"measured_runs",
		"upload-artifact",
	} {
		if strings.Contains(operationsText, forbidden) {
			t.Fatalf("Autback operations workflow retains superseded fragment %q", forbidden)
		}
	}
	if matches, err := filepath.Glob(filepath.Join(root, ".autback", "benchmarks", "*")); err != nil {
		t.Fatalf("inspect retired benchmark definitions: %v", err)
	} else if len(matches) != 0 {
		t.Fatalf("retired Autback benchmark definitions remain: %v", matches)
	}
	taskfile, err := os.ReadFile(filepath.Join(root, "Taskfile.yml"))
	if err != nil {
		t.Fatalf("read Taskfile: %v", err)
	}
	text := string(taskfile)
	for _, want := range []string{
		"ci:",
		"ci:local:",
		"autback:image:build:",
		"--cache go-build=/root/.cache/go-build",
		"--cache go-mod=/go/pkg/mod",
		"--cache bun=/root/.bun/install/cache",
		"autback exec",
		"-- task ci:local",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Taskfile missing generic Autback consumer fragment %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "AUTBACK.md")); !os.IsNotExist(err) {
		t.Fatalf("root Autback handover document must move into the architecture documentation: %v", err)
	}
	cutover, err := os.ReadFile(filepath.Join(root, "docs", "articles", "architecture", "autback.md"))
	if err != nil {
		t.Fatalf("read LeapView Autback architecture: %v", err)
	}
	cutoverText := string(cutover)
	for _, want := range []string{
		"[Autback](https://github.com/flidai/autback)",
		"Autback resolves project selection in this order",
		"`--project`, then `AUTBACK_PROJECT`",
		"task ci:local",
		"autback image rollback --project leapview",
		"repository@sha256",
		"GitHub environment `autback`",
		"External and Dependabot pull requests",
		"task ci",
	} {
		if !strings.Contains(cutoverText, want) {
			t.Fatalf("LeapView Autback architecture missing %q", want)
		}
	}
	navigation, err := os.ReadFile(filepath.Join(root, "docs", "navigation.yaml"))
	if err != nil {
		t.Fatalf("read documentation navigation: %v", err)
	}
	for _, want := range []string{
		"slug: architecture/autback",
		"source: articles/architecture/autback.md",
	} {
		if !strings.Contains(string(navigation), want) {
			t.Fatalf("documentation navigation missing Autback architecture fragment %q", want)
		}
	}
}

func TestPublicSiteBuildGeneratesIgnoredBrowserContracts(t *testing.T) {
	root := repoRoot(t)
	taskfile, err := os.ReadFile(filepath.Join(root, "Taskfile.yml"))
	if err != nil {
		t.Fatalf("read Taskfile.yml: %v", err)
	}
	block := taskfileTaskBlock(t, string(taskfile), "site:build")
	for _, dependency := range []string{
		"- task: ui-signals:generate",
		"- task: visualization-ir:generate",
	} {
		if !strings.Contains(block, dependency) {
			t.Errorf("site:build must generate ignored browser contract %q in a clean checkout", dependency)
		}
	}
}

func workflowJobBlock(t *testing.T, workflow, job string) string {
	t.Helper()
	startMarker := "  " + job + ":"
	lines := strings.Split(workflow, "\n")
	start := -1
	for index, line := range lines {
		if line == startMarker {
			start = index
			break
		}
	}
	if start < 0 {
		t.Fatalf("workflow job %q not found", job)
	}
	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		line := lines[index]
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(line, ":") {
			end = index
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

func taskfileTaskBlock(t *testing.T, taskfile, task string) string {
	t.Helper()
	startMarker := "  " + task + ":"
	lines := strings.Split(taskfile, "\n")
	start := -1
	for index, line := range lines {
		if line == startMarker {
			start = index
			break
		}
	}
	if start < 0 {
		t.Fatalf("Taskfile task %q not found", task)
	}
	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		line := lines[index]
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(line, ":") {
			end = index
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

func TestSQLCOutputsAreGeneratedBuildInputs(t *testing.T) {
	root := repoRoot(t)
	files := map[string][]string{
		"Taskfile.yml": {
			"db:generate:",
			"go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate",
			"- task: db:generate",
		},
		".gitignore": {
			"internal/platform/db/db.go",
			"internal/platform/db/models.go",
			"internal/platform/db/*.sql.go",
			"internal/*/internal/db/",
			"internal/platform/**/sqlite/*db/",
		},
		".dockerignore": {
			"internal/platform/db/db.go",
			"internal/platform/db/models.go",
			"internal/platform/db/*.sql.go",
			"internal/*/internal/db/",
			"internal/platform/**/sqlite/*db/",
		},
		filepath.Join("scripts", "generate_build_sources.sh"): {
			"go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate",
		},
		"Dockerfile": {
			"./scripts/generate_build_sources.sh",
			"FROM go-deps AS build",
			"COPY --from=sourcegen /src/internal/access/internal/db ./internal/access/internal/db",
			"COPY --from=sourcegen /src/internal/admin/internal/db ./internal/admin/internal/db",
			"COPY --from=sourcegen /src/internal/agent/internal/db ./internal/agent/internal/db",
			"COPY --from=sourcegen /src/internal/analytics/internal/db ./internal/analytics/internal/db",
			"COPY --from=sourcegen /src/internal/dashboard/internal/db ./internal/dashboard/internal/db",
			"COPY --from=sourcegen /src/internal/deployment/internal/db ./internal/deployment/internal/db",
			"COPY --from=sourcegen /src/internal/manageddata/internal/db ./internal/manageddata/internal/db",
			"COPY --from=sourcegen /src/internal/refresh/internal/db ./internal/refresh/internal/db",
			"COPY --from=sourcegen /src/internal/release/internal/db ./internal/release/internal/db",
			"COPY --from=sourcegen /src/internal/servingstate/internal/db ./internal/servingstate/internal/db",
			"COPY --from=sourcegen /src/internal/workspace/internal/db ./internal/workspace/internal/db",
			"COPY --from=sourcegen /src/internal/platform/http/cursorsigning/sqlite/cursordb ./internal/platform/http/cursorsigning/sqlite/cursordb",
			"COPY --from=sourcegen /src/internal/platform/http/idempotency/sqlite/idempotencydb ./internal/platform/http/idempotency/sqlite/idempotencydb",
			"COPY --from=sourcegen /src/internal/platform/jobs/sqlite/jobdb ./internal/platform/jobs/sqlite/jobdb",
		},
	}
	for name, fragments := range files {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, fragment := range fragments {
			if !strings.Contains(string(body), fragment) {
				t.Errorf("%s missing sqlc generation contract fragment %q", name, fragment)
			}
		}
	}
}

func TestDerivedArtifactsAreGeneratedBuildInputs(t *testing.T) {
	root := repoRoot(t)
	files := map[string][]string{
		".gitignore": {
			"internal/app/config/config_gen.go",
			"internal/app/config/spec/names_gen.go",
			"web/generated/",
			"docs/catalog.json",
			"docs/search-index.sqlite3",
			"docs/configuration.md",
			"docs/api/*.md",
			"docs/api/operations.json",
			"docs/reference/cli/",
			"docs/reference/config/",
		},
		".dockerignore": {
			"internal/app/config/config_gen.go",
			"internal/app/config/spec/names_gen.go",
			"web/generated",
			"docs/catalog.json",
			"docs/search-index.sqlite3",
			"docs/configuration.md",
			"docs/api/*.md",
			"docs/api/operations.json",
			"docs/reference/cli",
			"docs/reference/config",
		},
		"Dockerfile.site": {
			"AS go-deps",
			"FROM go-deps AS sourcegen",
			"./scripts/generate_build_sources.sh",
			"go run ./internal/app/tools/clidocgen",
			"go run ./internal/app/tools/schemadocgen",
			"go run ./internal/app/tools/openapidocgen",
			"go run -tags=duckdb_arrow ./internal/app/tools/visualdocgen",
			"go run ./internal/app/tools/docsitegen",
			"FROM sourcegen AS build",
			"COPY --from=sourcegen /src/web/generated ./web/generated",
		},
		"Dockerfile": {
			"FROM go-deps AS build",
			"COPY --from=sourcegen /src/internal/app/config/config_gen.go ./internal/app/config/config_gen.go",
			"COPY --from=sourcegen /src/internal/app/config/spec/names_gen.go ./internal/app/config/spec/names_gen.go",
			"COPY --from=sourcegen /src/web/generated ./web/generated",
		},
		filepath.Join("scripts", "generate_build_sources.sh"): {
			"go run ./internal/app/tools/configgen",
		},
		"Taskfile.yml": {
			"desc: Build the LeapView public site assets from generated contracts",
			"desc: Build the independently deployable public site from generated documentation",
			"desc: Start the public site from generated documentation on http://localhost:8081",
		},
	}
	for name, fragments := range files {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, fragment := range fragments {
			if !strings.Contains(string(body), fragment) {
				t.Errorf("%s missing generated-input contract fragment %q", name, fragment)
			}
		}
	}
	siteDockerfile, err := os.ReadFile(filepath.Join(root, "Dockerfile.site"))
	if err != nil {
		t.Fatalf("read Dockerfile.site: %v", err)
	}
	if count := strings.Count(string(siteDockerfile), "RUN go mod download"); count != 1 {
		t.Fatalf("Dockerfile.site downloads Go modules %d times, want one shared dependency stage", count)
	}
	const seededModuleCache = "type=cache,id=leapview-go-mod,target=/go/pkg/mod,from=go-deps,source=/go/pkg/mod,sharing=locked"
	if count := strings.Count(string(siteDockerfile), seededModuleCache); count != 3 {
		t.Fatalf("Dockerfile.site uses the seeded persistent Go module cache %d times, want source generation, visual documentation, and compilation", count)
	}

	gitignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	require.NoError(t, err)
	if strings.Contains(string(gitignore), "!docs/reference/cli/manifest.json") {
		t.Error("generated CLI manifest must not be exempted from Git ignore rules")
	}

	taskfile, err := os.ReadFile(filepath.Join(root, "Taskfile.yml"))
	require.NoError(t, err)
	for _, generated := range []string{"docs/api/operations.json", "docs/reference/cli/manifest.json"} {
		if strings.Contains(generatedCheckCommand(string(taskfile)), generated) {
			t.Errorf("generated:check treats build-only artifact %q as a public snapshot", generated)
		}
	}
}

func TestArrowResponseContractDeclaresCursorTrailer(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "api", "typespec", "common.tsp"))
	require.NoError(t, err)
	contract := string(body)
	for _, fragment := range []string{
		`@extension("x-leapview-response-trailers", #["X-Next-Cursor"])`,
		`@header("Trailer") trailers: "X-Next-Cursor";`,
	} {
		if !strings.Contains(contract, fragment) {
			t.Errorf("Arrow response contract missing trailer declaration %q", fragment)
		}
	}
	if strings.Contains(contract, `@header("X-Next-Cursor")`) {
		t.Error("Arrow response contract still advertises X-Next-Cursor as an initial header")
	}
	operations, err := os.ReadFile(filepath.Join(root, "api", "typespec", "bi.tsp"))
	require.NoError(t, err)
	if got := strings.Count(string(operations), `@extension("x-leapview-response-trailers", #["X-Next-Cursor"])`); got != 3 {
		t.Errorf("Arrow operation trailer declarations = %d, want 3", got)
	}
	openAPI, err := os.ReadFile(filepath.Join(root, "docs", "api", "openapi.yaml"))
	require.NoError(t, err)
	if got := strings.Count(string(openAPI), "x-leapview-response-trailers:"); got != 3 {
		t.Errorf("generated OpenAPI trailer declarations = %d, want 3", got)
	}
}

func workflowStep(workflow, startMarker, endMarker string) string {
	start := strings.Index(workflow, startMarker)
	if start < 0 {
		return ""
	}
	rest := workflow[start+len(startMarker):]
	end := strings.Index(rest, endMarker)
	if end < 0 {
		return rest
	}
	return rest[:end]
}

func generatedCheckCommand(taskfile string) string {
	start := strings.Index(taskfile, "  generated:check:")
	if start < 0 {
		return ""
	}
	rest := taskfile[start+len("  generated:check:"):]
	end := strings.Index(rest, "\n  api:generate:")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

func TestFixedPlatformSQLiteQueriesUseSQLC(t *testing.T) {
	root := repoRoot(t)
	queryContracts := map[string][]string{
		filepath.Join("internal", "access", "sqlite", "queries", "access.sql"): {
			"-- name: DeleteRoleGrantTemplates :exec",
			"-- name: InsertRoleGrantTemplate :exec",
		},
		filepath.Join("internal", "platform", "db", "queries", "platform.sql"): {
			"-- name: InsertPlatformSettingIfMissing :exec",
		},
		filepath.Join("internal", "manageddata", "sqlite", "queries", "managed_data.sql"): {
			"-- name: ListManagedDataReachabilitySources :many",
		},
	}
	for name, markers := range queryContracts {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, marker := range markers {
			if !strings.Contains(string(body), marker) {
				t.Errorf("%s missing sqlc query %q", name, marker)
			}
		}
	}

	handwrittenSQL := map[string][]string{
		filepath.Join("internal", "platform", "store.go"): {
			"DELETE FROM role_grant_templates",
			"INSERT INTO role_grant_templates",
			"INSERT INTO securable_objects",
			"INSERT INTO platform_settings",
		},
		filepath.Join("internal", "manageddata", "maintenance", "sqlite", "source.go"): {
			"const reachabilityQuery",
			"QueryContext(ctx, reachabilityQuery)",
		},
	}
	for name, fragments := range handwrittenSQL {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for _, fragment := range fragments {
			if strings.Contains(string(body), fragment) {
				t.Errorf("%s retains fixed-shape SQLite query %q instead of using sqlc", name, fragment)
			}
		}
	}
}

func TestAPIv1SQLiteAdaptersUseSQLC(t *testing.T) {
	packages := map[string]struct{}{
		"internal/platform/http/idempotency/sqlite":   {},
		"internal/jobs/sqlite":                        {},
		"internal/platform/http/cursorsigning/sqlite": {},
		"internal/release/sqlite":                     {},
	}
	for _, file := range productionGoFiles(t) {
		if _, ok := packages[file.pkgDir]; !ok {
			continue
		}
		for _, forbidden := range []string{".ExecContext(", ".QueryContext(", ".QueryRowContext("} {
			if strings.Contains(file.body, forbidden) {
				t.Errorf("%s bypasses sqlc via %s", file.path, forbidden)
			}
		}
	}
}

func TestStorageArchitectureSpecDocumentsProcessOwnedDuckDB(t *testing.T) {
	root := repoRoot(t)
	spec, err := os.ReadFile(filepath.Join(root, "docs", "storage-architecture-spec.md"))
	if err != nil {
		t.Fatalf("read storage architecture spec: %v", err)
	}
	text := string(spec)
	for _, want := range []string{
		"one process-owned DuckDB `DatabaseInstance`",
		"leapview.db               # node-local control-plane state",
		"ducklake/catalog.duckdb   # DuckDB-backed DuckLake metadata catalog",
		"Every physical relation in a serving plan",
		"AT (VERSION => 42)",
		"Runtime retirement closes generation-scoped cache state",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("storage architecture spec missing global catalog contract fragment %q", want)
		}
	}
	for _, forbidden := range []string{
		"ducklake:sqlite:",
		"PostgreSQL as the server/multi-user DuckLake catalog backend",
		"one DuckDB file per semantic model",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("storage architecture spec still contains obsolete shared-catalog contract fragment %q", forbidden)
		}
	}
}

func TestAnalyticsModuleConstructsTheProcessDuckDBExactlyOnce(t *testing.T) {
	constructors := []string{}
	for _, file := range productionGoFiles(t) {
		if file.pkgDir == "internal/analytics/module" && strings.Contains(file.body, "analyticsducklake.Open(") {
			constructors = append(constructors, file.path)
		}
	}
	if len(constructors) != 1 {
		t.Fatalf("analytics module constructs DuckDB in %v, want exactly one constructor", constructors)
	}
	root := repoRoot(t)
	for _, path := range []string{
		"internal/app/runtimefactory/factory.go",
		"internal/analytics/duckdb/materialize.go",
		"internal/refresh/analyticsruntime/materializer.go",
		"internal/dashboard/analyticsruntime/factory.go",
		"internal/runtimehost/manager.go",
	} {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		require.NoError(t, err)
		if strings.Contains(string(body), "analyticsducklake.Open(") || strings.Contains(string(body), "OpenSnapshot(") {
			t.Errorf("%s constructs a runtime-owned DuckDB instance", path)
		}
	}
}

func TestGovernedAnalyticalSessionBoundaryHasNoLegacyServingEscape(t *testing.T) {
	root := repoRoot(t)
	for _, path := range []string{
		"internal/analytics/ducklake/environment.go",
		"internal/dashboard/analyticsruntime/factory.go",
		"internal/dashboard/runtime/service.go",
		"internal/analytics/dataquery/query.go",
	} {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		require.NoError(t, err)
		text := string(body)
		for _, forbidden := range []string{"func (e *Environment) SQLDB(", "OpenMaterializeRuntime", "OpenDashboardDataRuntime", "KindSourceRows"} {
			if strings.Contains(text, forbidden) {
				t.Errorf("%s retains legacy analytical escape %q", path, forbidden)
			}
		}
	}
}

func TestCurrentConnectorRegistryExcludesFutureQuackProduct(t *testing.T) {
	root := repoRoot(t)
	for _, path := range []string{
		"internal/analytics/connectors/registry.go",
		"internal/project/schema/contracts/contracts.cue",
		"schemas/json/connection.schema.json",
	} {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		require.NoError(t, err)
		if strings.Contains(strings.ToLower(string(body)), "quack") {
			t.Errorf("%s exposes future Quack product as a current connector", path)
		}
	}
}

func TestProductionUIDoesNotDependOnCDNScripts(t *testing.T) {
	root := repoRoot(t)
	forbiddenHosts := []string{"cdn.jsdelivr.net", "unpkg.com", "esm.sh", "skypack.dev"}

	for _, dir := range []string{"internal/workspace/ui", "internal/dashboard/ui", "internal/app"} {
		err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(dir)), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text := string(body)
			for _, forbidden := range forbiddenHosts {
				if strings.Contains(text, forbidden) {
					rel, _ := filepath.Rel(root, path)
					t.Fatalf("%s references external script host %q; production UI assets must be served from /static", filepath.ToSlash(rel), forbidden)
				}
			}
			return nil
		})
		require.NoError(t, err)
	}

	staticFiles, err := filepath.Glob(filepath.Join(root, "static", "*.js"))
	require.NoError(t, err)
	for _, path := range staticFiles {
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		text := string(body)
		for _, forbidden := range forbiddenHosts {
			if strings.Contains(text, forbidden) {
				rel, _ := filepath.Rel(root, path)
				t.Fatalf("%s references external asset host %q; production bundles must be self-contained", filepath.ToSlash(rel), forbidden)
			}
		}
	}
}

func isSQLDBAllowedFile(file goFile) bool {
	if rule, ok := ClassifyPackage(file.pkgDir); ok && (rule.Layer == LayerComposition || rule.Layer == LayerModule) {
		return true
	}
	if file.pkgDir == "internal/app" {
		switch file.path {
		case "internal/app/build.go",
			"internal/app/server.go",
			"internal/app/publishes.go",
			"internal/app/refresh_runs.go",
			"internal/app/query_audit.go":
			return true
		default:
			return false
		}
	}
	if file.pkgDir == "internal/app/cli" ||
		file.pkgDir == "internal/integration" ||
		strings.HasPrefix(file.pkgDir, "internal/admin/storage") ||
		strings.HasPrefix(file.pkgDir, "internal/analytics/duckdb") ||
		strings.HasPrefix(file.pkgDir, "internal/analytics/ducklake") ||
		strings.HasSuffix(file.pkgDir, "/sqlite") ||
		strings.Contains(file.pkgDir, "/sqlite/") {
		return true
	}
	return false
}

func importListContains(imports []string, value string) bool {
	for _, imported := range imports {
		if imported == value || strings.Contains(imported, value) {
			return true
		}
	}
	return false
}

func productionGoFiles(t *testing.T) []goFile {
	t.Helper()
	root := repoRoot(t)
	files := []goFile{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			// Self-contained tools may live in the monorepo while retaining their own
			// module and architecture. The LeapView package rules stop at that module
			// boundary just as the Go toolchain does.
			if path != root {
				if _, statErr := os.Stat(filepath.Join(path, "go.mod")); statErr == nil {
					return filepath.SkipDir
				}
			}
			switch entry.Name() {
			case ".git", "node_modules":
				return filepath.SkipDir
			}
			if filepath.Dir(path) == root {
				switch entry.Name() {
				case "static", "web", "dashboards":
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		imports := make([]string, 0, len(parsed.Imports))
		for _, imported := range parsed.Imports {
			imports = append(imports, strings.Trim(imported.Path.Value, `"`))
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, goFile{
			path:    rel,
			pkgDir:  strings.TrimSuffix(rel, "/"+filepath.Base(rel)),
			imports: imports,
			body:    string(body),
		})
		return nil
	})
	require.NoError(t, err)
	return files
}

func packageDirExists(root, dir string) bool {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		return true
	}
	return false
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatal("go.mod not found")
		}
		dir = next
	}
}

func isInternalPackage(pkgDir string) bool {
	return pkgDir == "internal" || strings.HasPrefix(pkgDir, "internal/")
}

func isAdapterOrCompositionPackage(pkgDir string) bool {
	if rule, ok := ClassifyPackage(pkgDir); ok {
		switch rule.Layer {
		case LayerAdapter, LayerModule, LayerComposition, LayerPlatform:
			return true
		}
	}
	if pkgDir == "internal/app" ||
		pkgDir == "internal/app/cli" ||
		pkgDir == "internal/integration" ||
		pkgDir == "internal/platform" ||
		strings.HasPrefix(pkgDir, "internal/platform/") ||
		pkgDir == "internal/analytics/resource" ||
		pkgDir == "internal/access/oidc" ||
		pkgDir == "internal/access/httpauth" ||
		pkgDir == "internal/access/scimprov" ||
		pkgDir == "internal/admin/storage" ||
		pkgDir == "internal/agent/tools" ||
		strings.HasPrefix(pkgDir, "internal/app/tools/") ||
		strings.Contains(pkgDir, "/testing/") {
		return true
	}
	if strings.HasSuffix(pkgDir, "/module") {
		return true
	}
	for _, suffix := range []string{"/http", "/sqlite", "/filesystem", "/s3", "/tus", "/duckdb", "/ducklake", "/datastar", "/openai", "/ui"} {
		if strings.HasSuffix(pkgDir, suffix) || strings.Contains(pkgDir, suffix+"/") {
			return true
		}
	}
	return false
}

func isForbiddenUseCaseImport(imported string) bool {
	if imported == "net/http" ||
		imported == "database/sql" ||
		imported == "github.com/go-chi/chi/v5" ||
		strings.Contains(imported, "datastar") ||
		strings.Contains(imported, "gomponents") {
		return true
	}
	if imported == modulePath+"/internal/platform/db" {
		return true
	}
	if !strings.HasPrefix(imported, modulePath+"/internal/") {
		return false
	}
	packagePath := strings.TrimPrefix(imported, modulePath+"/")
	if rule, ok := ClassifyPackage(packagePath); ok && rule.Layer == LayerPlatform {
		return false
	}
	for _, segment := range []string{"/sqlite", "/filesystem", "/s3", "/tus", "/duckdb", "/ducklake", "/datastar", "/http", "/openai"} {
		if strings.Contains(packagePath, segment) {
			return true
		}
	}
	return false
}
