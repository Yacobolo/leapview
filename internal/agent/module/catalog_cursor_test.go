package module

import (
	"errors"
	"testing"

	agenttools "github.com/Yacobolo/leapview/internal/agent/tools"
	"github.com/Yacobolo/leapview/internal/platform/http/cursorsigning"
)

func TestCatalogListCursorBindsScopeRequestAndSnapshot(t *testing.T) {
	service := CatalogService{signCursor: cursorsigning.Sign, verifyCursor: cursorsigning.Verify}
	scope := agenttools.Scope{PrincipalID: "p1"}
	request := agenttools.CatalogListRequest{Limit: 1}
	items := []agenttools.CatalogItem{
		{Ref: agenttools.CatalogRef{WorkspaceID: "a", Type: agenttools.CatalogTypeWorkspace, ID: "a"}, Name: "A"},
		{Ref: agenttools.CatalogRef{WorkspaceID: "b", Type: agenttools.CatalogTypeWorkspace, ID: "b"}, Name: "B"},
	}
	snapshot := catalogItemsSnapshot(items)
	cursor := service.encodeCatalogListCursor(scope, request, snapshot, 1)
	if offset, err := service.decodeCatalogListCursor(cursor, scope, request, snapshot); err != nil || offset != 1 {
		t.Fatalf("decode cursor = %d, %v", offset, err)
	}
	if _, err := service.decodeCatalogListCursor(cursor, agenttools.Scope{PrincipalID: "p2"}, request, snapshot); err == nil {
		t.Fatal("cursor accepted a different principal")
	}
	if _, err := service.decodeCatalogListCursor(cursor, scope, request, catalogItemsSnapshot(items[:1])); err == nil {
		t.Fatal("cursor accepted a changed snapshot")
	} else {
		var catalogErr *agenttools.CatalogError
		if !errors.As(err, &catalogErr) || catalogErr.Code != "catalog_snapshot_changed" {
			t.Fatalf("snapshot error = %v", err)
		}
	}
	metadataChanged := append([]agenttools.CatalogItem(nil), items...)
	metadataChanged[1].Description = "Changed after the first page"
	if _, err := service.decodeCatalogListCursor(cursor, scope, request, catalogItemsSnapshot(metadataChanged)); err == nil {
		t.Fatal("cursor accepted changed item metadata")
	}
}
