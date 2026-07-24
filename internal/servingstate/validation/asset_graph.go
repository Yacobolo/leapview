// Package validation defines the narrow immutable asset-graph envelope stored
// and checked at serving-state boundaries.
package validation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type Asset struct {
	ID             string
	SnapshotID     string
	WorkspaceID    string
	ServingStateID string
	Type           string
	Key            string
	ParentID       string
	Title          string
	Description    string
	SourceFile     string `json:"sourceFile,omitempty"`
	PayloadSchema  string
	PayloadJSON    string
	ContentHash    string
}

type AssetEdge struct {
	ID             string
	WorkspaceID    string
	ServingStateID string
	FromAssetID    string
	ToAssetID      string
	Type           string
}

type AssetGraph struct {
	Assets []Asset
	Edges  []AssetEdge
}

func ConvertAssetGraph(value any) (AssetGraph, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return AssetGraph{}, err
	}
	return DecodeAssetGraph(data)
}

func NewAssetEdge(workspaceID, servingStateID, from, to, typ string) AssetEdge {
	return AssetEdge{
		ID: edgeID(servingStateID, from, to, typ), WorkspaceID: workspaceID, ServingStateID: servingStateID,
		FromAssetID: from, ToAssetID: to, Type: typ,
	}
}

func DecodeAssetGraph(data []byte) (AssetGraph, error) {
	var value AssetGraph
	err := json.Unmarshal(data, &value)
	return value, err
}

func ValidateAssetGraph(graph AssetGraph, workspaceID, servingStateID string) error {
	assets := make(map[string]struct{}, len(graph.Assets))
	for _, asset := range graph.Assets {
		if asset.ID == "" {
			return fmt.Errorf("asset logical id is required")
		}
		if _, exists := assets[asset.ID]; exists {
			return fmt.Errorf("asset %s is duplicated", asset.ID)
		}
		assets[asset.ID] = struct{}{}
		if asset.WorkspaceID != workspaceID {
			return fmt.Errorf("asset %s workspace = %q, want %q", asset.ID, asset.WorkspaceID, workspaceID)
		}
		if asset.ServingStateID != servingStateID {
			return fmt.Errorf("asset %s serving state = %q, want %q", asset.ID, asset.ServingStateID, servingStateID)
		}
		if want := assetSnapshotID(servingStateID, asset.ID); asset.SnapshotID != want {
			return fmt.Errorf("asset %s snapshot id = %q, want %q", asset.ID, asset.SnapshotID, want)
		}
		if asset.SourceFile == "" {
			return fmt.Errorf("asset %s source file is required", asset.ID)
		}
	}
	for _, asset := range graph.Assets {
		if asset.ParentID == "" {
			continue
		}
		if _, ok := assets[asset.ParentID]; !ok {
			return fmt.Errorf("asset %s parent %s is not in graph", asset.ID, asset.ParentID)
		}
	}
	edges := map[string]struct{}{}
	for _, edge := range graph.Edges {
		if edge.WorkspaceID != workspaceID || edge.ServingStateID != servingStateID {
			return fmt.Errorf("asset edge %s has mismatched scope", edge.ID)
		}
		if _, ok := assets[edge.FromAssetID]; !ok {
			return fmt.Errorf("asset edge %s from asset %s is not in graph", edge.ID, edge.FromAssetID)
		}
		if _, ok := assets[edge.ToAssetID]; !ok {
			return fmt.Errorf("asset edge %s to asset %s is not in graph", edge.ID, edge.ToAssetID)
		}
		want := edgeID(servingStateID, edge.FromAssetID, edge.ToAssetID, edge.Type)
		if edge.ID != want {
			return fmt.Errorf("asset edge %s id = %q, want %q", edge.Type, edge.ID, want)
		}
		key := edge.FromAssetID + "|" + edge.ToAssetID + "|" + edge.Type
		if _, exists := edges[key]; exists {
			return fmt.Errorf("asset edge %s is duplicated", edge.ID)
		}
		edges[key] = struct{}{}
	}
	return nil
}

func assetSnapshotID(servingStateID, assetID string) string {
	return "asset_" + stableID(servingStateID+"|"+assetID)
}

func edgeID(servingStateID, from, to, typ string) string {
	return "edge_" + stableID(servingStateID+"|"+from+"|"+to+"|"+typ)
}

func stableID(value string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(value)))
	return hex.EncodeToString(sum[:])[:32]
}
