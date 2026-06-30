package workspace

import "sort"

var knownAgentTools = map[string]struct{}{
	"asset_lineage":                 {},
	"describe_asset":                {},
	"describe_dashboard":            {},
	"describe_dashboard_visual":     {},
	"explain_semantic_preview":      {},
	"explain_semantic_query":        {},
	"get_deployment":                {},
	"get_materialization_run":       {},
	"list_assets":                   {},
	"list_dashboard_components":     {},
	"list_dashboard_filter_options": {},
	"list_dashboards":               {},
	"list_deployments":              {},
	"list_materialization_runs":     {},
	"list_semantic_datasets":        {},
	"list_semantic_fields":          {},
	"list_semantic_models":          {},
	"list_workspace_asset_edges":    {},
	"list_workspaces":               {},
	"preview_semantic_dataset":      {},
	"query_dashboard_page":          {},
	"query_dashboard_table_data":    {},
	"query_dashboard_visual_data":   {},
	"query_semantic_dataset":        {},
	"query_table":                   {},
	"query_visual":                  {},
	"search_workspace":              {},
	"describe_model":                {},
	"describe_semantic_dataset":     {},
}

func IsKnownAgentTool(name string) bool {
	_, ok := knownAgentTools[name]
	return ok
}

func KnownAgentToolNames() []string {
	out := make([]string, 0, len(knownAgentTools))
	for name := range knownAgentTools {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func DefaultAgentPolicy() AgentPolicy {
	return AgentPolicy{Enabled: true}
}
