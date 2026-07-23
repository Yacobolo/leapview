package architecture

import "strings"

// Layer describes the architectural role of a Go package. The classification
// is intentionally data, rather than test code, so every architecture check
// evaluates the same package model.
type Layer string

const (
	LayerContract    Layer = "contract"
	LayerUseCase     Layer = "use_case"
	LayerAdapter     Layer = "adapter"
	LayerModule      Layer = "module"
	LayerComposition Layer = "composition"
	LayerPlatform    Layer = "platform"
)

// PackageRule assigns an internal package prefix to one accountable owner and
// layer. The longest matching prefix wins, which permits narrow transitional
// ownership declarations without weakening the general capability rule.
type PackageRule struct {
	Prefix     string
	Capability string
	Layer      Layer
}

// PublicContractPrefixes declares the only packages another capability may
// import as a synchronous contract. Adapter and module packages are never
// made public merely because their capability has an allowed edge.
var PublicContractPrefixes = map[string][]string{
	"access":       {"internal/access"},
	"analytics":    {"internal/analytics/model", "internal/analytics/query", "internal/analytics/materialize", "internal/analytics/connectors", "internal/analytics/arrowquery", "internal/analytics/resource", "internal/analytics/queryaudit", "internal/dataquery", "internal/queryruntime"},
	"dashboard":    {"internal/dashboard", "internal/dashboard/report", "internal/visualization/definition", "internal/visualization/format", "internal/visualization/geometry", "internal/visualization/ir", "internal/visualization/mapasset", "internal/visualization/runtime"},
	"manageddata":  {"internal/manageddata", "internal/manageddata/binding"},
	"workspace":    {"internal/workspace", "internal/search"},
	"project":      {"internal/configschema", "internal/configspec", "internal/project/artifact", "internal/project/bundle", "internal/project/compiler"},
	"release":      {"internal/release"},
	"deployment":   {"internal/deployment"},
	"servingstate": {"internal/servingstate"},
	"refresh":      {"internal/refresh/artifact", "internal/refresh/plan", "internal/refresh/run", "internal/refresh/schedule"},
	"runtimehost":  {"internal/runtimehost"},
	"workload":     {"internal/workload"},
}

// DeferredPackageEdges are the explicit ownership exceptions retained for
// later roadmap slices. They are package-scoped so a deferred compiler,
// refresh-state, or persistence concern cannot silently authorize the same
// edge from unrelated production code.
type DeferredPackageEdge struct {
	SourcePrefix string
	Target       string
	RoadmapSlice int
}

var DeferredPackageEdges = []DeferredPackageEdge{}

var WorkloadImportPrefixes = []string{
	"internal/app", "internal/cli", "internal/config", "internal/integration", "internal/tools",
	"internal/platform/jobs", "internal/admin/storage", "internal/refresh/run",
	"internal/admin/module",
	"internal/dashboard/module",
	"internal/refresh/module",
	"internal/servingstate/module",
	"internal/workload/module",
	"internal/analytics/materialize", "internal/analytics/ducklake", "internal/dashboard/semanticapi",
}

func AllowsWorkloadImport(packagePath string) bool {
	for _, prefix := range WorkloadImportPrefixes {
		if packagePath == prefix || strings.HasPrefix(packagePath, prefix+"/") {
			return true
		}
	}
	return false
}

func IsDeferredPackageEdge(sourcePath, targetCapability string) bool {
	for _, edge := range DeferredPackageEdges {
		if edge.Target == targetCapability && (sourcePath == edge.SourcePrefix || strings.HasPrefix(sourcePath, edge.SourcePrefix+"/")) {
			return true
		}
	}
	return false
}

var CapabilityDependencies = map[string]map[string]bool{
	"project":      {"workspace": true, "analytics": true, "dashboard": true, "access": true, "refresh": true, "servingstate": true},
	"workspace":    {"access": true, "analytics": true, "dashboard": true},
	"access":       {},
	"manageddata":  {"servingstate": true},
	"analytics":    {"access": true, "manageddata": true, "servingstate": true},
	"dashboard":    {"access": true, "analytics": true, "runtimehost": true, "workload": true},
	"agent":        {"access": true, "analytics": true, "dashboard": true, "workspace": true, "project": true},
	"admin":        {"analytics": true},
	"release":      {"project": true, "workspace": true, "servingstate": true},
	"deployment":   {"access": true, "release": true, "servingstate": true, "manageddata": true, "runtimehost": true},
	"servingstate": {"workload": true},
	"refresh":      {"access": true, "servingstate": true, "manageddata": true, "analytics": true, "runtimehost": true, "workspace": true, "workload": true},
	"runtimehost":  {"manageddata": true, "servingstate": true},
	"workload":     {},
	"platform":     {"project": true, "workload": true},
}

func IsPublicContractImport(capability, packagePath string) bool {
	for _, prefix := range PublicContractPrefixes[capability] {
		if packagePath == prefix || strings.HasPrefix(packagePath, prefix+"/") {
			return true
		}
	}
	return false
}

var PackageRules = []PackageRule{
	{Prefix: "cmd", Capability: "composition", Layer: LayerComposition},
	{Prefix: "docs", Capability: "ui", Layer: LayerAdapter},
	{Prefix: "site", Capability: "ui", Layer: LayerAdapter},
	{Prefix: "pkg/agent", Capability: "agent", Layer: LayerContract},
	{Prefix: "pkg/pagestream", Capability: "ui", Layer: LayerAdapter},
	{Prefix: "internal/project/compiler", Capability: "project", Layer: LayerUseCase},
	{Prefix: "internal/project/artifact", Capability: "project", Layer: LayerContract},
	{Prefix: "internal/refresh/plan", Capability: "refresh", Layer: LayerUseCase},
	{Prefix: "internal/refresh/run", Capability: "refresh", Layer: LayerUseCase},
	{Prefix: "internal/refresh/schedule", Capability: "refresh", Layer: LayerUseCase},
	{Prefix: "internal/apiidempotency", Capability: "platform", Layer: LayerAdapter},
	{Prefix: "internal/api/transport", Capability: "api", Layer: LayerAdapter},
	{Prefix: "internal/api/protocol", Capability: "api", Layer: LayerAdapter},
	{Prefix: "internal/architecture", Capability: "platform", Layer: LayerPlatform},
	{Prefix: "internal/assetnav", Capability: "workspace", Layer: LayerUseCase},
	{Prefix: "internal/brand", Capability: "ui", Layer: LayerAdapter},
	{Prefix: "internal/catalog", Capability: "platform", Layer: LayerPlatform},
	{Prefix: "internal/cli", Capability: "composition", Layer: LayerComposition},
	{Prefix: "internal/composectl", Capability: "project", Layer: LayerAdapter},
	{Prefix: "internal/config", Capability: "platform", Layer: LayerPlatform},
	{Prefix: "internal/configschema", Capability: "project", Layer: LayerContract},
	{Prefix: "internal/configspec", Capability: "project", Layer: LayerContract},
	{Prefix: "internal/cursorsigning", Capability: "platform", Layer: LayerAdapter},
	{Prefix: "internal/dataquery", Capability: "analytics", Layer: LayerContract},
	{Prefix: "internal/docvalidation", Capability: "project", Layer: LayerUseCase},
	{Prefix: "internal/instancelock", Capability: "platform", Layer: LayerAdapter},
	{Prefix: "internal/observability", Capability: "platform", Layer: LayerAdapter},
	{Prefix: "internal/queryruntime", Capability: "dashboard", Layer: LayerContract},
	{Prefix: "internal/search", Capability: "workspace", Layer: LayerUseCase},
	{Prefix: "internal/secret", Capability: "platform", Layer: LayerPlatform},
	{Prefix: "internal/securefs", Capability: "platform", Layer: LayerPlatform},
	{Prefix: "internal/snapshot", Capability: "platform", Layer: LayerPlatform},
	{Prefix: "internal/site", Capability: "ui", Layer: LayerAdapter},
	{Prefix: "internal/staticasset", Capability: "ui", Layer: LayerAdapter},
	{Prefix: "internal/storage", Capability: "platform", Layer: LayerAdapter},
	{Prefix: "internal/testutil", Capability: "platform", Layer: LayerAdapter},
	{Prefix: "internal/tools", Capability: "composition", Layer: LayerComposition},
	{Prefix: "internal/visualization/definition", Capability: "dashboard", Layer: LayerContract},
	{Prefix: "internal/visualization/format", Capability: "dashboard", Layer: LayerContract},
	{Prefix: "internal/visualization/geometry", Capability: "dashboard", Layer: LayerContract},
	{Prefix: "internal/visualization/ir", Capability: "dashboard", Layer: LayerContract},
	{Prefix: "internal/visualization/mapasset", Capability: "dashboard", Layer: LayerContract},
	{Prefix: "internal/visualization/mapasset/http", Capability: "dashboard", Layer: LayerAdapter},
	{Prefix: "internal/visualization/runtime", Capability: "dashboard", Layer: LayerUseCase},
	{Prefix: "internal/visualdocs", Capability: "ui", Layer: LayerAdapter},
	// Cross-capability infrastructure bridges are process composition, not
	// dashboard or refresh domain dependencies.
	{Prefix: "internal/dashboard/analyticsduckdb", Capability: "composition", Layer: LayerComposition},
	{Prefix: "internal/dashboard/semanticapi", Capability: "dashboard", Layer: LayerAdapter},
	{Prefix: "internal/refresh/analyticsduckdb", Capability: "composition", Layer: LayerComposition},
	{Prefix: "internal/app", Capability: "composition", Layer: LayerComposition},
	{Prefix: "internal/admin", Capability: "admin", Layer: LayerAdapter},
	{Prefix: "internal/api", Capability: "api", Layer: LayerContract},
	{Prefix: "internal/ui", Capability: "ui", Layer: LayerAdapter},
	{Prefix: "internal/ui/transport", Capability: "ui", Layer: LayerAdapter},
}

var adapterSegments = []string{
	"/http", "/sqlite", "/filesystem", "/s3", "/tus", "/duckdb", "/ducklake", "/datastar", "/openai", "/ui",
}

// ClassifyPackage returns the owner and layer for an internal package path.
func ClassifyPackage(path string) (PackageRule, bool) {
	path = strings.TrimSuffix(path, "/")
	best := PackageRule{}
	found := false
	for _, rule := range PackageRules {
		if path != rule.Prefix && !strings.HasPrefix(path, rule.Prefix+"/") {
			continue
		}
		if !found || len(rule.Prefix) > len(best.Prefix) {
			best, found = rule, true
		}
	}
	if found {
		if best.Layer != LayerComposition && strings.HasSuffix(path, "/module") {
			best.Layer = LayerModule
		}
		return best, true
	}
	const internal = "internal/"
	if !strings.HasPrefix(path, internal) {
		return PackageRule{}, false
	}
	remainder := strings.TrimPrefix(path, internal)
	owner := strings.SplitN(remainder, "/", 2)[0]
	if _, ok := CapabilityDependencies[owner]; !ok {
		return PackageRule{}, false
	}
	layer := LayerUseCase
	if path == internal+owner {
		layer = LayerContract
	}
	if strings.HasSuffix(path, "/module") {
		layer = LayerModule
	} else {
		for _, segment := range adapterSegments {
			if strings.Contains(path, segment) {
				layer = LayerAdapter
				break
			}
		}
	}
	return PackageRule{Prefix: internal + owner, Capability: owner, Layer: layer}, true
}

func HasExplicitPackageRule(path string) bool {
	for _, rule := range PackageRules {
		if path == rule.Prefix || strings.HasPrefix(path, rule.Prefix+"/") {
			return true
		}
	}
	return false
}
