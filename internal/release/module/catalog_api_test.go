package module

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yacobolo/leapview/internal/release"
	releaseapi "github.com/Yacobolo/leapview/internal/release/api"
)

type catalogRepository struct {
	projects []release.ProjectRecord
}

func (r catalogRepository) ListProjects(context.Context) ([]release.ProjectRecord, error) {
	return r.projects, nil
}
func (catalogRepository) GetProject(context.Context, string) (release.ProjectRecord, error) {
	return release.ProjectRecord{}, nil
}
func (catalogRepository) ListProjectWorkspaces(context.Context, string, string) ([]release.WorkspaceRecord, error) {
	return nil, nil
}
func (catalogRepository) ListConnections(context.Context, string, string) ([]release.ConnectionRecord, error) {
	return nil, nil
}
func (catalogRepository) GetConnection(context.Context, string, string, string) (release.ConnectionRecord, error) {
	return release.ConnectionRecord{}, nil
}

func TestReleaseModuleOwnsProjectCatalogMapping(t *testing.T) {
	module := &Module{catalog: catalogRepository{projects: []release.ProjectRecord{{
		ID: "project-1", CreatedAt: "2026-07-23T12:00:00Z", UpdatedAt: "2026-07-23T13:00:00Z",
		LatestReleaseID: "release-1", ActiveDeploymentID: "deployment-1",
	}}}}
	recorder := httptest.NewRecorder()
	module.ListProjects(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil), releaseapi.PageParams{})
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	for _, expected := range []string{`"id":"project-1"`, `"latestReleaseId":"release-1"`, `"activeDeploymentId":"deployment-1"`} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Fatalf("body = %s, missing %s", recorder.Body.String(), expected)
		}
	}
}
