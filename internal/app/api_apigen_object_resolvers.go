package app

import (
	"github.com/Yacobolo/libredash/internal/access/httpauth"
	agenthttp "github.com/Yacobolo/libredash/internal/agent/http"
	queryhttp "github.com/Yacobolo/libredash/internal/analytics/query/http"
	dashboardhttp "github.com/Yacobolo/libredash/internal/dashboard/http"
	workspacehttp "github.com/Yacobolo/libredash/internal/workspace/http"
)

// Object resolvers remain handwritten because they bind generated operation
// metadata to request-aware domain behavior rather than declarative policy.
var apigenOperationObjectResolvers = map[string]httpauth.ObjectResolver{
	"getWorkspaceAsset":          workspacehttp.AssetObjectRefs,
	"getWorkspaceAssetLineage":   workspacehttp.AssetObjectRefs,
	"listWorkspaceAssetEdges":    workspacehttp.AssetObjectRefs,
	"getDashboard":               dashboardhttp.DashboardObjectRefs,
	"listDashboardComponents":    dashboardhttp.DashboardObjectRefs,
	"getDashboardVisual":         dashboardhttp.DashboardObjectRefs,
	"queryDashboardPage":         dashboardhttp.DashboardObjectRefs,
	"queryDashboardVisualData":   dashboardhttp.DashboardObjectRefs,
	"queryDashboardTable":        dashboardhttp.DashboardObjectRefs,
	"queryDashboardTableData":    dashboardhttp.DashboardObjectRefs,
	"listDashboardFilterOptions": dashboardhttp.DashboardObjectRefs,
	"getSemanticModel":           queryhttp.SemanticDatasetObjectRefs,
	"listSemanticModelFields":    queryhttp.SemanticDatasetObjectRefs,
	"querySemanticModel":         queryhttp.SemanticDatasetObjectRefs,
	"explainSemanticModelQuery":  queryhttp.SemanticDatasetObjectRefs,
	"listSemanticDatasets":       queryhttp.SemanticDatasetObjectRefs,
	"getSemanticDataset":         queryhttp.SemanticDatasetObjectRefs,
	"listSemanticFields":         queryhttp.SemanticDatasetObjectRefs,
	"querySemanticDataset":       queryhttp.SemanticDatasetObjectRefs,
	"previewSemanticDataset":     queryhttp.SemanticDatasetObjectRefs,
	"explainSemanticQuery":       queryhttp.SemanticDatasetObjectRefs,
	"explainSemanticPreview":     queryhttp.SemanticDatasetObjectRefs,
	"getAgentConversation":       agenthttp.ConversationObjectRefs,
	"updateAgentConversation":    agenthttp.ConversationObjectRefs,
	"archiveAgentConversation":   agenthttp.ConversationObjectRefs,
	"listAgentMessages":          agenthttp.ConversationObjectRefs,
	"createAgentTurn":            agenthttp.ConversationObjectRefs,
	"listAgentRuns":              agenthttp.ConversationObjectRefs,
	"getAgentRun":                agenthttp.ConversationObjectRefs,
	"listAgentEvents":            agenthttp.ConversationObjectRefs,
}
