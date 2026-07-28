package release

// FinalizationEventData returns the durable public event projection for a
// terminal release finalization transition.
func FinalizationEventData(row Release) map[string]any {
	workspaces := make([]map[string]any, 0, len(row.Manifest.Workspaces))
	for _, item := range row.Manifest.Workspaces {
		mapped := map[string]any{"workspace": item.WorkspaceID, "artifactDigest": item.ArtifactDigest}
		if item.ServingStateID != "" {
			mapped["servingStateId"] = item.ServingStateID
		}
		workspaces = append(workspaces, mapped)
	}
	connections := make([]map[string]any, 0, len(row.Manifest.Connections))
	for _, item := range row.Manifest.Connections {
		connections = append(connections, map[string]any{"connection": item.ConnectionID, "revisionId": item.RevisionID})
	}
	result := map[string]any{
		"id": row.ID, "projectId": row.ProjectID, "projectDigest": row.ProjectDigest,
		"status": string(row.Status), "createdBy": row.CreatedBy, "createdAt": row.CreatedAt,
		"workspaces": workspaces, "connections": connections,
	}
	if row.FinalizedAt != "" {
		result["finalizedAt"] = row.FinalizedAt
	}
	if row.Error != "" {
		result["error"] = row.Error
	}
	return result
}
