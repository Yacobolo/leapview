package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type WorkspaceID string
type DeploymentID string
type AssetID string
type AssetEdgeID string

func NewAssetID(deploymentID DeploymentID, typ AssetType, key string) AssetID {
	return AssetID("asset_" + stableID(string(deploymentID)+"|"+string(typ)+"|"+key))
}

func NewAssetEdgeID(deploymentID DeploymentID, fromID, toID AssetID, typ AssetEdgeType) AssetEdgeID {
	return AssetEdgeID("edge_" + stableID(string(deploymentID)+"|"+string(fromID)+"|"+string(toID)+"|"+string(typ)))
}

func stableID(value string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(value)))
	return hex.EncodeToString(sum[:])[:32]
}
