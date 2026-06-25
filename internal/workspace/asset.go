package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type Asset struct {
	ID            AssetID
	SnapshotID    AssetSnapshotID
	WorkspaceID   WorkspaceID
	DeploymentID  DeploymentID
	Type          AssetType
	Key           string
	ParentID      AssetID
	Title         string
	Description   string
	PayloadSchema string
	PayloadJSON   string
	ContentHash   string
}

type AssetEdge struct {
	ID           AssetEdgeID
	WorkspaceID  WorkspaceID
	DeploymentID DeploymentID
	FromAssetID  AssetID
	ToAssetID    AssetID
	Type         AssetEdgeType
}

type AssetGraph struct {
	Assets []Asset
	Edges  []AssetEdge
}

func NewAsset(workspaceID WorkspaceID, deploymentID DeploymentID, typ AssetType, key string, parentID AssetID, title, description, payloadSchema string, payload any) (Asset, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return Asset{}, err
	}
	id := NewAssetID(typ, key)
	hashBytes, err := json.Marshal(assetHashPayload{
		Type:          typ,
		Key:           key,
		ParentID:      parentID,
		Title:         title,
		Description:   description,
		PayloadSchema: payloadSchema,
		PayloadJSON:   json.RawMessage(payloadBytes),
	})
	if err != nil {
		return Asset{}, err
	}
	sum := sha256.Sum256(hashBytes)
	return Asset{
		ID:            id,
		SnapshotID:    NewAssetSnapshotID(deploymentID, id),
		WorkspaceID:   workspaceID,
		DeploymentID:  deploymentID,
		Type:          typ,
		Key:           key,
		ParentID:      parentID,
		Title:         title,
		Description:   description,
		PayloadSchema: payloadSchema,
		PayloadJSON:   string(payloadBytes),
		ContentHash:   hex.EncodeToString(sum[:]),
	}, nil
}

type assetHashPayload struct {
	Type          AssetType       `json:"type"`
	Key           string          `json:"key"`
	ParentID      AssetID         `json:"parentId,omitempty"`
	Title         string          `json:"title"`
	Description   string          `json:"description"`
	PayloadSchema string          `json:"payloadSchema"`
	PayloadJSON   json.RawMessage `json:"payload"`
}

func NewAssetEdge(workspaceID WorkspaceID, deploymentID DeploymentID, fromID, toID AssetID, typ AssetEdgeType) AssetEdge {
	return AssetEdge{
		ID:           NewAssetEdgeID(deploymentID, fromID, toID, typ),
		WorkspaceID:  workspaceID,
		DeploymentID: deploymentID,
		FromAssetID:  fromID,
		ToAssetID:    toID,
		Type:         typ,
	}
}
