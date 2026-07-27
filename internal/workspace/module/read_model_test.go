package module

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apigenapi "github.com/Yacobolo/leapview/internal/app/api/gen"
	"github.com/Yacobolo/leapview/internal/workspace"
)

type activeMetadataReadModel struct {
	graphCalls int
}

func (r *activeMetadataReadModel) List(context.Context) ([]workspace.Summary, error) {
	return []workspace.Summary{{ID: "sales", Title: "stale", Description: "stale"}}, nil
}

func (r *activeMetadataReadModel) ByID(context.Context, workspace.WorkspaceID) (workspace.Summary, error) {
	return workspace.Summary{ID: "sales", Title: "stale", Description: "stale"}, nil
}

func (r *activeMetadataReadModel) ActiveServingStateGraph(context.Context, workspace.WorkspaceID, string) (workspace.AssetGraph, bool, error) {
	r.graphCalls++
	return workspace.AssetGraph{}, false, nil
}

func (r *activeMetadataReadModel) AssetVersions(context.Context, workspace.WorkspaceID, string, workspace.AssetID) ([]workspace.AssetVersion, error) {
	return nil, nil
}

func (r *activeMetadataReadModel) ListWithActiveMetadata(context.Context, string) ([]workspace.Summary, error) {
	return []workspace.Summary{{
		ID: "sales", Title: "Active Sales", Description: "from active catalog",
		ActiveServingStateID: "dep_active",
	}}, nil
}

func (r *activeMetadataReadModel) ByIDWithActiveMetadata(context.Context, workspace.WorkspaceID, string) (workspace.Summary, error) {
	return workspace.Summary{
		ID: "sales", Title: "Active Sales", Description: "from active catalog",
		ActiveServingStateID: "dep_active",
	}, nil
}

func TestWorkspaceListUsesActiveMetadataWithoutLoadingGraphs(t *testing.T) {
	readModel := &activeMetadataReadModel{}
	module, err := Build(t.Context(), Config{
		ReadModel: readModel,
		Environment: func(*http.Request) string {
			return "dev"
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	module.HTTP().Workspaces(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/workspaces?environment=dev", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Items []apigenapi.WorkspaceResponse `json:"items"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode workspaces response: %v body=%s", err, recorder.Body.String())
	}
	if len(body.Items) != 1 {
		t.Fatalf("workspace count = %d body=%s", len(body.Items), recorder.Body.String())
	}
	got := body.Items[0]
	if got.Title != "Active Sales" || got.Description != "from active catalog" || got.ActiveServingStateId == nil || *got.ActiveServingStateId != "dep_active" {
		t.Fatalf("workspace = %#v, want active read-model metadata", got)
	}
	if readModel.graphCalls != 0 {
		t.Fatalf("ActiveServingStateGraph calls = %d, want 0", readModel.graphCalls)
	}
}
