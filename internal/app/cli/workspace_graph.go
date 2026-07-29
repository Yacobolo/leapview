package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flidai/leapview/internal/platform/cliapi"
	"github.com/flidai/leapview/internal/workspace"
	workspacegen "github.com/flidai/leapview/internal/workspace/api/gen"
)

// workspaceActiveGraphLoader adapts the Workspace capability's generated
// client to the Project CLI's consumer-owned planning port.
type workspaceActiveGraphLoader struct {
	client cliapi.Client
}

func (loader workspaceActiveGraphLoader) LoadActiveWorkspaceGraph(
	ctx context.Context,
	credentials cliapi.Credentials,
	assertedEnvironment string,
	workspaceID string,
) (workspace.AssetGraph, error) {
	if loader.client == nil {
		return workspace.AssetGraph{}, fmt.Errorf("Workspace CLI API client is required")
	}
	if _, err := loader.client.Environment(ctx, credentials, assertedEnvironment); err != nil {
		return workspace.AssetGraph{}, err
	}
	transport, err := loader.client.Transport(ctx, credentials)
	if err != nil {
		return workspace.AssetGraph{}, err
	}
	response, err := workspacegen.NewGenClient(transport).GetWorkspaceActiveAssetGraph(
		ctx,
		workspacegen.GenGetWorkspaceActiveAssetGraphClientRequest{Workspace: workspaceID},
	)
	if err != nil {
		return workspace.AssetGraph{}, err
	}
	return mapWorkspaceActiveGraph(response.Body), nil
}

func mapWorkspaceActiveGraph(response workspacegen.GenSchemaWorkspaceAssetGraphResponse) workspace.AssetGraph {
	graph := workspace.AssetGraph{
		Assets: make([]workspace.Asset, 0, len(response.Assets)),
		Edges:  make([]workspace.AssetEdge, 0, len(response.Edges)),
	}
	for _, asset := range response.Assets {
		graph.Assets = append(graph.Assets, workspace.Asset{
			ID:             workspace.AssetID(asset.Id),
			SnapshotID:     workspace.AssetSnapshotID(asset.SnapshotId),
			WorkspaceID:    workspace.WorkspaceID(asset.WorkspaceId),
			ServingStateID: workspace.ServingStateID(asset.ServingStateId),
			Type:           workspace.AssetType(asset.Type),
			Key:            asset.Key,
			ParentID:       workspace.AssetID(optionalWorkspaceGraphString(asset.ParentId)),
			Title:          asset.Title,
			Description:    asset.Description,
			PayloadSchema:  asset.PayloadSchema,
			SourceFile:     optionalWorkspaceGraphString(asset.SourceFile),
			PayloadJSON:    workspaceGraphPayloadJSON(asset.Payload),
			ContentHash:    asset.ContentHash,
		})
	}
	for _, edge := range response.Edges {
		graph.Edges = append(graph.Edges, workspace.AssetEdge{
			ID:             workspace.AssetEdgeID(edge.Id),
			WorkspaceID:    workspace.WorkspaceID(edge.WorkspaceId),
			ServingStateID: workspace.ServingStateID(edge.ServingStateId),
			FromAssetID:    workspace.AssetID(edge.FromAssetId),
			ToAssetID:      workspace.AssetID(edge.ToAssetId),
			Type:           workspace.AssetEdgeType(edge.Type),
		})
	}
	return graph
}

func optionalWorkspaceGraphString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func workspaceGraphPayloadJSON(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(encoded)
}
