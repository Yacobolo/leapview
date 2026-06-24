package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/Yacobolo/libredash/internal/api"
	"github.com/Yacobolo/libredash/internal/deployment"
	"github.com/Yacobolo/libredash/internal/deployment/activate"
	deploymentfs "github.com/Yacobolo/libredash/internal/deployment/filesystem"
	deploymentsqlite "github.com/Yacobolo/libredash/internal/deployment/sqlite"
	"github.com/Yacobolo/libredash/internal/deployment/validate"
	"github.com/Yacobolo/libredash/internal/platform"
	"github.com/Yacobolo/libredash/internal/workspace"
	"github.com/go-chi/chi/v5"
)

type runtimeReloader interface {
	Reload(ctx context.Context) error
	PrepareDeployment(ctx context.Context, deploymentID string) (deployment.PreparedRuntime, error)
	CommitPrepared(prepared deployment.PreparedRuntime) error
}

type deploymentRepository interface {
	validate.Repository
	activate.Repository
	Create(ctx context.Context, input deployment.CreateInput) (deployment.Deployment, error)
	List(ctx context.Context, workspaceID deployment.WorkspaceID) ([]deployment.Deployment, error)
}

func (s *Server) createDeployment(w http.ResponseWriter, r *http.Request) {
	var input api.DeploymentCreateRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&input)
	}
	workspaceID := s.workspaceID(input.WorkspaceID)
	workspaceRepo, err := s.workspaceRepository()
	if err != nil {
		writeJSONError(w, err, http.StatusInternalServerError)
		return
	}
	if workspaceRepo == nil {
		writeJSONError(w, fmt.Errorf("workspace repository is not configured"), http.StatusInternalServerError)
		return
	}
	if err := workspaceRepo.Ensure(r.Context(), workspace.EnsureInput{ID: workspace.WorkspaceID(workspaceID), Title: firstNonEmpty(input.Title, workspaceID), Description: input.Description}); err != nil {
		writeJSONError(w, err, http.StatusInternalServerError)
		return
	}
	repo, err := s.deploymentRepository()
	if err != nil {
		writeJSONError(w, err, http.StatusInternalServerError)
		return
	}
	createdBy := ""
	if s.auth != nil {
		if principal, ok := s.auth.Principal(r); ok {
			createdBy = principal.ID
		}
	}
	deployment, err := repo.Create(r.Context(), deployment.CreateInput{WorkspaceID: deployment.WorkspaceID(workspaceID), CreatedBy: createdBy})
	if err != nil {
		writeJSONError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, deploymentDTO(deployment))
}

func (s *Server) uploadDeploymentArtifact(w http.ResponseWriter, r *http.Request) {
	deploymentID := chi.URLParam(r, "deployment")
	repo, err := s.deploymentRepository()
	if err != nil {
		writeJSONError(w, err, http.StatusInternalServerError)
		return
	}
	deployment, err := repo.ByID(r.Context(), deployment.ID(deploymentID))
	if err != nil {
		writeJSONError(w, err, statusForNotFound(err))
		return
	}
	if err := os.MkdirAll(s.artifactDir, 0o755); err != nil {
		writeJSONError(w, err, http.StatusInternalServerError)
		return
	}
	artifactStore := deploymentfs.NewArtifactStore(s.artifactDir)
	path := artifactStore.UploadPath(deployment.ID)
	out, err := os.Create(path)
	if err != nil {
		writeJSONError(w, err, http.StatusInternalServerError)
		return
	}
	size, copyErr := io.Copy(out, http.MaxBytesReader(w, r.Body, 128<<20))
	closeErr := out.Close()
	if copyErr != nil {
		writeJSONError(w, copyErr, http.StatusBadRequest)
		return
	}
	if closeErr != nil {
		writeJSONError(w, closeErr, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deploymentId": deployment.ID, "sizeBytes": size})
}

func (s *Server) validateDeployment(w http.ResponseWriter, r *http.Request) {
	deploymentID := chi.URLParam(r, "deployment")
	repo, err := s.deploymentRepository()
	if err != nil {
		writeJSONError(w, err, http.StatusInternalServerError)
		return
	}
	service := validate.NewService(repo, deploymentfs.NewArtifactStore(s.artifactDir), deploymentfs.Validator{})
	deployment, err := service.Validate(r.Context(), deployment.ID(deploymentID))
	if err != nil {
		writeJSONError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, deploymentDTO(deployment))
}

func (s *Server) activateDeployment(w http.ResponseWriter, r *http.Request) {
	deploymentID := chi.URLParam(r, "deployment")
	repo, err := s.deploymentRepository()
	if err != nil {
		writeJSONError(w, err, http.StatusInternalServerError)
		return
	}
	service := activate.NewService(repo, s.reloader)
	deployment, err := service.Activate(r.Context(), deployment.ID(deploymentID))
	if err != nil {
		writeJSONError(w, err, statusForActivationError(err))
		return
	}
	writeJSON(w, http.StatusOK, deploymentDTO(deployment))
}

func (s *Server) listDeployments(w http.ResponseWriter, r *http.Request) {
	workspaceID := s.workspaceID(r.URL.Query().Get("workspace"))
	repo, err := s.deploymentRepository()
	if err != nil {
		writeJSONError(w, err, http.StatusInternalServerError)
		return
	}
	rows, err := repo.List(r.Context(), deployment.WorkspaceID(workspaceID))
	if err != nil {
		writeJSONError(w, err, http.StatusInternalServerError)
		return
	}
	response := make([]api.DeploymentResponse, 0, len(rows))
	for _, row := range rows {
		response = append(response, deploymentDTO(row))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) getDeployment(w http.ResponseWriter, r *http.Request) {
	repo, err := s.deploymentRepository()
	if err != nil {
		writeJSONError(w, err, http.StatusInternalServerError)
		return
	}
	deployment, err := repo.ByID(r.Context(), deployment.ID(chi.URLParam(r, "deployment")))
	if err != nil {
		writeJSONError(w, err, statusForNotFound(err))
		return
	}
	writeJSON(w, http.StatusOK, deploymentDTO(deployment))
}

func (s *Server) rollbackDeployment(w http.ResponseWriter, r *http.Request) {
	s.activateDeployment(w, r)
}

func (s *Server) workspaceID(candidate string) string {
	if candidate != "" {
		return candidate
	}
	if s.defaultWorkspaceID != "" {
		return s.defaultWorkspaceID
	}
	return platform.DefaultWorkspaceID
}

func (s *Server) deploymentRepository() (deploymentRepository, error) {
	if s.deploymentRepo != nil {
		return s.deploymentRepo, nil
	}
	if s.store == nil {
		return nil, fmt.Errorf("deployment repository is not configured")
	}
	s.deploymentRepo = deploymentsqlite.NewRepository(s.store.SQLDB())
	return s.deploymentRepo, nil
}

func deploymentDTO(row deployment.Deployment) api.DeploymentResponse {
	out := api.DeploymentResponse{
		ID:          string(row.ID),
		WorkspaceID: string(row.WorkspaceID),
		Status:      string(row.Status),
		Digest:      row.Digest,
		CreatedAt:   row.CreatedAt,
		Error:       row.Error,
	}
	out.ActivatedAt = row.ActivatedAt
	return out
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(w http.ResponseWriter, err error, status int) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func statusForNotFound(err error) int {
	if err == sql.ErrNoRows || errors.Is(err, deployment.ErrNotFound) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

func statusForActivationError(err error) int {
	if errors.Is(err, deployment.ErrNotFound) {
		return http.StatusNotFound
	}
	if errors.Is(err, activate.ErrInvalidStatus) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
