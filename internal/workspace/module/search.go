package module

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Yacobolo/leapview/internal/access"
	"github.com/Yacobolo/leapview/internal/platform/http/cursorsigning"
	api "github.com/Yacobolo/leapview/internal/platform/http/model"
	workspaceapi "github.com/Yacobolo/leapview/internal/workspace/api"
	productsearch "github.com/Yacobolo/leapview/internal/workspace/search"
	searchsqlite "github.com/Yacobolo/leapview/internal/workspace/search/sqlite"
)

type searchService interface {
	Search(context.Context, productsearch.Subject, productsearch.Query) (productsearch.Page, error)
}

type SearchParams = workspaceapi.SearchParams

type searchAuthorizer struct {
	authorize func(context.Context, string, access.Privilege, access.ObjectRef) (bool, error)
}

func buildSearch(database *sql.DB, authorize func(context.Context, string, access.Privilege, access.ObjectRef) (bool, error)) searchService {
	if database == nil {
		return nil
	}
	return productsearch.NewService(searchsqlite.New(database), searchAuthorizer{authorize: authorize}, searchCursorSigner{})
}

type searchCursorSigner struct{}

func (searchCursorSigner) Sign(prefix string, payload []byte) string {
	return cursorsigning.Sign(prefix, payload)
}

func (searchCursorSigner) Verify(prefix, token string) ([]byte, error) {
	return cursorsigning.Verify(prefix, token)
}

func (a searchAuthorizer) CanView(ctx context.Context, subject productsearch.Subject, object access.ObjectRef) (bool, error) {
	if subject.CredentialRestricted && !containsSearchPrivilege(subject.Privileges, access.PrivilegeViewItem) {
		return false, nil
	}
	if subject.DevBypass {
		return true, nil
	}
	if subject.Restricted {
		allowedWorkspace := false
		for _, workspaceID := range subject.WorkspaceIDs {
			if strings.TrimSpace(workspaceID) == object.WorkspaceID {
				allowedWorkspace = true
				break
			}
		}
		if !allowedWorkspace {
			return false, nil
		}
	}
	if a.authorize == nil {
		return false, nil
	}
	return a.authorize(ctx, subject.ID, access.PrivilegeViewItem, object)
}

func (m *Module) SearchSubject(r *http.Request) (productsearch.Subject, bool) {
	if m == nil || m.handler.ReadModel.CurrentPrincipal == nil {
		return productsearch.Subject{}, false
	}
	principal, ok := m.handler.ReadModel.CurrentPrincipal(r)
	if !ok {
		return productsearch.Subject{}, false
	}
	subject := productsearch.Subject{ID: principal.ID, DevBypass: principal.DevBypass}
	if m.currentCredential != nil {
		if credential, ok := m.currentCredential(r); ok {
			subject.ID = credential.Principal.ID
			subject.DevBypass = false
			subject.CredentialRestricted = credential.Token.Privileges != nil
			if credential.Token.ID != "" {
				subject.CredentialID = "token:" + credential.Token.ID
			}
			for _, privilege := range credential.Token.Privileges {
				subject.Privileges = append(subject.Privileges, string(privilege))
			}
			if workspaceID := strings.TrimSpace(credential.Token.WorkspaceID); workspaceID != "" {
				subject.Restricted = true
				subject.WorkspaceIDs = []string{workspaceID}
			}
		}
	}
	return subject, true
}

func (m *Module) Search(ctx context.Context, subject productsearch.Subject, query productsearch.Query) (productsearch.Page, error) {
	if m == nil || m.search == nil {
		return productsearch.Page{}, errors.New("search is not configured")
	}
	return m.search.Search(ctx, subject, query)
}

func (m *Module) ResolveSearchReferences(ctx context.Context, subject productsearch.Subject, environment string, references []productsearch.Reference) ([]productsearch.Result, error) {
	if m == nil || m.search == nil {
		return nil, errors.New("search is not configured")
	}
	service, ok := m.search.(interface {
		Resolve(context.Context, productsearch.Subject, string, []productsearch.Reference) ([]productsearch.Result, error)
	})
	if !ok {
		return nil, errors.New("search reference resolution is not configured")
	}
	return service.Resolve(ctx, subject, environment, references)
}

func (m *Module) SearchAPI(w http.ResponseWriter, r *http.Request, params workspaceapi.SearchParams) {
	subject, ok := m.SearchSubject(r)
	if !ok {
		writeSearchJSONError(w, errors.New("search principal is unavailable"), http.StatusUnauthorized)
		return
	}
	environment := ""
	if m.handler.Environment != nil {
		environment = m.handler.Environment(r)
	}
	query := productsearch.Query{Environment: environment}
	if params.Query != nil {
		query.Text = *params.Query
	}
	if params.Workspaces != nil {
		query.Workspaces = append([]string(nil), (*params.Workspaces)...)
	}
	if params.Types != nil {
		query.Types = make([]productsearch.Type, 0, len(*params.Types))
		for _, typ := range *params.Types {
			query.Types = append(query.Types, productsearch.Type(typ))
		}
	}
	if params.ContextWorkspace != nil {
		query.Context.WorkspaceID = *params.ContextWorkspace
	}
	if params.ContextDashboard != nil {
		query.Context.DashboardID = *params.ContextDashboard
	}
	if params.ContextPage != nil {
		query.Context.PageID = *params.ContextPage
	}
	if params.Limit != nil {
		query.Limit = int(*params.Limit)
	}
	if params.PageToken != nil {
		query.Cursor = *params.PageToken
	}
	page, err := m.Search(r.Context(), subject, query)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, productsearch.ErrInvalidCursor) {
			status = http.StatusBadRequest
		} else if errors.Is(err, productsearch.ErrSnapshotChanged) {
			status = http.StatusConflict
		} else if strings.Contains(err.Error(), "unknown search type") || strings.Contains(err.Error(), "search limit") {
			status = http.StatusBadRequest
		}
		writeSearchJSONError(w, err, status)
		return
	}
	writeSearchJSON(w, http.StatusOK, workspaceapi.SearchResponse{Items: searchAPIResults(page.Items), Page: workspaceapi.PageInfo{NextCursor: searchStringPointer(page.NextCursor)}})
}

func searchAPIResults(items []productsearch.Result) []workspaceapi.SearchResult {
	out := make([]workspaceapi.SearchResult, 0, len(items))
	for _, item := range items {
		locations := make([]workspaceapi.SearchLocation, 0, len(item.Locations))
		for _, location := range item.Locations {
			locations = append(locations, workspaceapi.SearchLocation{
				DashboardID: searchOptionalString(location.DashboardID), DashboardName: searchOptionalString(location.DashboardName),
				PageID: searchOptionalString(location.PageID), PageName: searchOptionalString(location.PageName), Href: location.Href,
			})
		}
		contextTags := make([]workspaceapi.SearchContextTag, 0, len(item.Context))
		for _, tag := range item.Context {
			contextTags = append(contextTags, workspaceapi.SearchContextTag(tag))
		}
		out = append(out, workspaceapi.SearchResult{
			Reference: workspaceapi.SearchReference{WorkspaceID: item.Reference.WorkspaceID, Type: string(item.Reference.Type), ID: item.Reference.ID},
			Name:      item.Name, Description: searchOptionalString(item.Description),
			VisualType: searchOptionalString(item.VisualType),
			Workspace:  workspaceapi.SearchWorkspaceSummary{ID: item.Workspace.ID, Name: item.Workspace.Name},
			Href:       item.Href, Locations: locations, Context: contextTags,
		})
	}
	return out
}

func containsSearchPrivilege(privileges []string, wanted access.Privilege) bool {
	for _, privilege := range privileges {
		if strings.EqualFold(strings.TrimSpace(privilege), string(wanted)) {
			return true
		}
	}
	return false
}

func searchOptionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func searchStringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func writeSearchJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeSearchJSONError(w http.ResponseWriter, err error, status int) {
	writeSearchJSON(w, status, api.ErrorResponse{Code: status, Message: err.Error(), Details: map[string]any{}, RequestID: ""})
}
