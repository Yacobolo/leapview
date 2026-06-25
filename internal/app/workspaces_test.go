package app

import (
	"testing"

	"github.com/Yacobolo/libredash/internal/workspace"
)

func TestAssetDTOUsesLogicalIDAndTypedPayload(t *testing.T) {
	asset, err := workspace.NewAsset(
		workspace.WorkspaceID("test"),
		workspace.DeploymentID("deploy_a"),
		workspace.AssetTypeVisual,
		"executive-sales.orders",
		workspace.AssetID("dashboard:executive-sales"),
		"Orders",
		"Orders visual",
		"visual.v1",
		map[string]any{"query_kind": "aggregate"},
	)
	if err != nil {
		t.Fatalf("asset: %v", err)
	}

	dto := assetDTOFromWorkspace(asset)
	if dto.ID != "visual:executive-sales.orders" || dto.SnapshotID == "" || dto.SnapshotID == dto.ID {
		t.Fatalf("asset identity = %#v", dto)
	}
	if dto.ParentID != "dashboard:executive-sales" || dto.PayloadSchema != "visual.v1" || dto.Payload["query_kind"] != "aggregate" {
		t.Fatalf("asset dto = %#v", dto)
	}
}
