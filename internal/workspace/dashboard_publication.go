package workspace

// DashboardPublication is the immutable, compiled authorization boundary for
// one anonymously published dashboard. DependencyAssetIDs is the complete
// transitive graph reachable from the dashboard in this serving state.
type DashboardPublication struct {
	Name                string   `json:"name"`
	Dashboard           string   `json:"dashboard"`
	DefaultPage         string   `json:"defaultPage"`
	AllowedOrigins      []string `json:"allowedOrigins,omitempty"`
	DependencyAssetIDs  []string `json:"dependencyAssetIds"`
	ConfigurationDigest string   `json:"configurationDigest"`
}
