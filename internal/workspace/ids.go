package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type WorkspaceID string
type DeploymentID string
type AssetID string
type AssetSnapshotID string
type AssetEdgeID string

func NewAssetID(typ AssetType, key string) AssetID {
	return AssetID(strings.ToLower(string(typ) + ":" + strings.TrimSpace(key)))
}

func NewAssetSnapshotID(deploymentID DeploymentID, logicalID AssetID) AssetSnapshotID {
	return AssetSnapshotID("asset_" + stableID(string(deploymentID)+"|"+string(logicalID)))
}

func NewAssetEdgeID(deploymentID DeploymentID, fromID, toID AssetID, typ AssetEdgeType) AssetEdgeID {
	return AssetEdgeID("edge_" + stableID(string(deploymentID)+"|"+string(fromID)+"|"+string(toID)+"|"+string(typ)))
}

func stableID(value string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(value)))
	return hex.EncodeToString(sum[:])[:32]
}
