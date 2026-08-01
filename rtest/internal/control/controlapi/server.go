package controlapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/flidai/leapview/rtest/internal/control"
	"github.com/flidai/leapview/rtest/internal/control/pki"
	controlsqlite "github.com/flidai/leapview/rtest/internal/control/sqlite"
	rtestv1 "github.com/flidai/leapview/rtest/internal/gen/rtest/v1"
	"github.com/flidai/leapview/rtest/internal/gen/rtest/v1/rtestv1connect"
	"github.com/flidai/leapview/rtest/internal/protocol"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const Version = "0.6.0"

type OIDCVerifier interface {
	Verify(context.Context, string) (control.GitHubClaims, error)
}

type Config struct {
	Store               *controlsqlite.Store
	Scheduler           control.Scheduler
	Authority           *pki.Authority
	OIDCVerifier        OIDCVerifier
	CASEndpoint         string
	CASInstance         string
	BuildKitEndpoint    string
	CredentialTTL       time.Duration
	AllowUnpinnedImages bool
}

type Server struct {
	config Config
}

func New(config Config) (http.Handler, error) {
	if config.Store == nil || config.Scheduler == nil || config.Authority == nil {
		return nil, errors.New("control store, scheduler, and certificate authority are required")
	}
	if config.CASEndpoint == "" || config.CASInstance == "" || config.BuildKitEndpoint == "" {
		return nil, errors.New("CAS endpoint, CAS instance, and BuildKit endpoint are required")
	}
	if config.CredentialTTL == 0 {
		config.CredentialTTL = 15 * time.Minute
	}
	if config.CredentialTTL < time.Minute || config.CredentialTTL > 2*time.Hour {
		return nil, errors.New("credential TTL must be between 1 minute and 2 hours")
	}
	server := &Server{config: config}
	path, handler := rtestv1connect.NewControlServiceHandler(server)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.HandleFunc("GET /healthz", func(response http.ResponseWriter, _ *http.Request) { response.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /readyz", func(response http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
		defer cancel()
		if err := config.Store.Check(ctx); err != nil {
			http.Error(response, "control store unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := config.Scheduler.Check(ctx); err != nil {
			http.Error(response, "worker unavailable", http.StatusServiceUnavailable)
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	return securityHeaders(mux), nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Cache-Control", "no-store")
		response.Header().Set("Strict-Transport-Security", "max-age=31536000")
		next.ServeHTTP(response, request)
	})
}

func (s *Server) GetServiceInfo(context.Context, *connect.Request[rtestv1.GetServiceInfoRequest]) (*connect.Response[rtestv1.GetServiceInfoResponse], error) {
	return connect.NewResponse(&rtestv1.GetServiceInfoResponse{Version: Version, Capabilities: []string{
		"connect", "projects", "project-images", "device-tokens", "device-enrollment", "github-oidc", "reapi-cas-mtls", "swarm-jobs", "buildkit-mtls",
	}}), nil
}

func (s *Server) CreateUser(ctx context.Context, request *connect.Request[rtestv1.CreateUserRequest]) (*connect.Response[rtestv1.CreateUserResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	user, err := s.config.Store.CreateUser(ctx, principal, request.Msg.Name, request.Msg.Admin)
	if err != nil {
		return nil, connectError(err)
	}
	if err := s.config.Store.Audit(ctx, principal, "", "user.create", user.ID, nil); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&rtestv1.CreateUserResponse{User: userProto(user)}), nil
}

func (s *Server) CreateProject(ctx context.Context, request *connect.Request[rtestv1.CreateProjectRequest]) (*connect.Response[rtestv1.CreateProjectResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	project, err := s.config.Store.CreateProject(ctx, principal, request.Msg.Slug, request.Msg.Name)
	if err != nil {
		return nil, connectError(err)
	}
	if err := s.config.Store.AddProjectMember(ctx, principal, project.ID, principal.UserID); err != nil {
		return nil, connectError(err)
	}
	if err := s.config.Store.Audit(ctx, principal, project.ID, "project.create", project.ID, nil); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&rtestv1.CreateProjectResponse{Project: projectProto(project)}), nil
}

func (s *Server) ListProjects(ctx context.Context, request *connect.Request[rtestv1.ListProjectsRequest]) (*connect.Response[rtestv1.ListProjectsResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	projects, err := s.config.Store.ListProjects(ctx, principal)
	if err != nil {
		return nil, connectError(err)
	}
	response := &rtestv1.ListProjectsResponse{Projects: make([]*rtestv1.Project, 0, len(projects))}
	for _, project := range projects {
		response.Projects = append(response.Projects, projectProto(project))
	}
	return connect.NewResponse(response), nil
}

func (s *Server) AddProjectMember(ctx context.Context, request *connect.Request[rtestv1.AddProjectMemberRequest]) (*connect.Response[rtestv1.AddProjectMemberResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	project, err := s.config.Store.AuthorizeProject(ctx, principal, request.Msg.Project)
	if err != nil {
		return nil, connectError(err)
	}
	if err := s.config.Store.AddProjectMember(ctx, principal, project.ID, request.Msg.UserId); err != nil {
		return nil, connectError(err)
	}
	if err := s.config.Store.Audit(ctx, principal, project.ID, "project.member.add", request.Msg.UserId, nil); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&rtestv1.AddProjectMemberResponse{}), nil
}

func (s *Server) ActivateProjectImage(ctx context.Context, request *connect.Request[rtestv1.ActivateProjectImageRequest]) (*connect.Response[rtestv1.ActivateProjectImageResponse], error) {
	principal, project, err := s.projectConfiguration(ctx, request.Header(), request.Msg.Project)
	if err != nil {
		return nil, err
	}
	if !imageDigestPattern.MatchString(request.Msg.Image) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("image must be pinned by sha256 digest"))
	}
	if err := s.config.Scheduler.ValidateImage(ctx, request.Msg.Image); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("validate project image: %w", err))
	}
	project, err = s.config.Store.ActivateProjectImage(ctx, principal, project.ID, request.Msg.Image)
	if err != nil {
		return nil, connectError(err)
	}
	if err := s.config.Store.Audit(ctx, principal, project.ID, "project.image.activate", project.ID, map[string]string{"image": project.ActiveImage, "replaced_image": project.PreviousImage}); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&rtestv1.ActivateProjectImageResponse{Project: projectProto(project)}), nil
}

func (s *Server) RollbackProjectImage(ctx context.Context, request *connect.Request[rtestv1.RollbackProjectImageRequest]) (*connect.Response[rtestv1.RollbackProjectImageResponse], error) {
	principal, project, err := s.projectConfiguration(ctx, request.Header(), request.Msg.Project)
	if err != nil {
		return nil, err
	}
	if project.PreviousImage == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("project has no previous image"))
	}
	if err := s.config.Scheduler.ValidateImage(ctx, project.PreviousImage); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("validate rollback image: %w", err))
	}
	project, err = s.config.Store.RollbackProjectImage(ctx, principal, project.ID)
	if err != nil {
		return nil, connectError(err)
	}
	if err := s.config.Store.Audit(ctx, principal, project.ID, "project.image.rollback", project.ID, map[string]string{"image": project.ActiveImage, "replaced_image": project.PreviousImage}); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&rtestv1.RollbackProjectImageResponse{Project: projectProto(project)}), nil
}

func (s *Server) SetProjectImagePolicy(ctx context.Context, request *connect.Request[rtestv1.SetProjectImagePolicyRequest]) (*connect.Response[rtestv1.SetProjectImagePolicyResponse], error) {
	principal, project, err := s.projectConfiguration(ctx, request.Header(), request.Msg.Project)
	if err != nil {
		return nil, err
	}
	project, err = s.config.Store.SetProjectImagePolicy(ctx, principal, project.ID, request.Msg.AllowImageOverrides)
	if err != nil {
		return nil, connectError(err)
	}
	if err := s.config.Store.Audit(ctx, principal, project.ID, "project.image.policy", project.ID, map[string]string{"allow_overrides": fmt.Sprint(project.AllowImageOverrides)}); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&rtestv1.SetProjectImagePolicyResponse{Project: projectProto(project)}), nil
}

func (s *Server) ListProjectImageHistory(ctx context.Context, request *connect.Request[rtestv1.ListProjectImageHistoryRequest]) (*connect.Response[rtestv1.ListProjectImageHistoryResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	project, err := s.config.Store.AuthorizeProject(ctx, principal, request.Msg.Project)
	if err != nil {
		return nil, connectError(err)
	}
	events, err := s.config.Store.ListProjectImageHistory(ctx, project.ID)
	if err != nil {
		return nil, connectError(err)
	}
	response := &rtestv1.ListProjectImageHistoryResponse{Events: make([]*rtestv1.ProjectImageEvent, 0, len(events))}
	for _, event := range events {
		response.Events = append(response.Events, projectImageEventProto(event))
	}
	return connect.NewResponse(response), nil
}

func (s *Server) projectConfiguration(ctx context.Context, header http.Header, selector string) (control.Principal, control.Project, error) {
	principal, err := s.authenticate(ctx, header)
	if err != nil {
		return control.Principal{}, control.Project{}, err
	}
	project, err := s.config.Store.AuthorizeProject(ctx, principal, selector)
	if err != nil {
		return control.Principal{}, control.Project{}, connectError(err)
	}
	if principal.Kind != control.PrincipalDevice || !principal.Admin {
		return control.Principal{}, control.Project{}, connectError(control.ErrForbidden)
	}
	return principal, project, nil
}

func (s *Server) PrepareJob(ctx context.Context, request *connect.Request[rtestv1.PrepareJobRequest]) (*connect.Response[rtestv1.PrepareJobResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	project, err := s.config.Store.AuthorizeProject(ctx, principal, request.Msg.Project)
	if err != nil {
		return nil, connectError(err)
	}
	message := *request.Msg
	if message.Image == "" {
		message.Image = project.ActiveImage
	} else if project.ActiveImage != "" && message.Image != project.ActiveImage && !project.AllowImageOverrides {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("project image overrides are disabled"))
	}
	if message.Image == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("project has no active runner image; activate one or pass --image while overrides are enabled"))
	}
	input, err := s.validateJob(project.ID, &message)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	requestHash, err := admissionHash(input)
	if err != nil {
		return nil, connectError(err)
	}
	job, replayed, err := s.config.Store.CreatePreparedJob(ctx, input, control.Idempotency{Key: request.Msg.IdempotencyKey, RequestHash: requestHash})
	if err != nil {
		return nil, connectError(err)
	}
	credential, err := s.config.Authority.Issue(pki.OperationJob, job.ID, s.config.CredentialTTL)
	if err != nil {
		if !replayed {
			_, _ = s.config.Store.FailJob(ctx, job.ID, "issue CAS credential")
		}
		return nil, connectError(err)
	}
	if !replayed {
		if err := s.config.Store.Audit(ctx, principal, project.ID, "job.prepare", job.ID, map[string]string{"image": job.Image}); err != nil {
			return nil, connectError(err)
		}
	}
	return connect.NewResponse(&rtestv1.PrepareJobResponse{Job: jobProto(job), Cas: connectionProto(s.config.CASEndpoint, s.config.CASInstance, credential)}), nil
}

func (s *Server) StartJob(ctx context.Context, request *connect.Request[rtestv1.StartJobRequest]) (*connect.Response[rtestv1.StartJobResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	job, err := s.authorizedJob(ctx, principal, request.Msg.Id)
	if err != nil {
		return nil, err
	}
	if !rootDigestPattern.MatchString(request.Msg.RootDigest) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("root digest must use REAPI hash/size form"))
	}
	job.RootDigest = request.Msg.RootDigest
	job.Status = protocol.StatusQueued
	if err := s.config.Scheduler.Create(ctx, job); err != nil {
		_, _ = s.config.Store.FailJob(ctx, job.ID, err.Error())
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("schedule job: %w", err))
	}
	job, err = s.config.Store.StartJob(ctx, job.ID, job.RootDigest)
	if err != nil {
		_ = s.config.Scheduler.Cancel(ctx, job.ID)
		return nil, connectError(err)
	}
	if err := s.config.Store.Audit(ctx, principal, job.ProjectID, "job.start", job.ID, map[string]string{"root_digest": job.RootDigest}); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&rtestv1.StartJobResponse{Job: jobProto(job)}), nil
}

func (s *Server) GetJob(ctx context.Context, request *connect.Request[rtestv1.GetJobRequest]) (*connect.Response[rtestv1.GetJobResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	job, err := s.authorizedJob(ctx, principal, request.Msg.Id)
	if err != nil {
		return nil, err
	}
	job, err = s.refreshJob(ctx, job)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&rtestv1.GetJobResponse{Job: jobProto(job)}), nil
}

func (s *Server) ListJobs(ctx context.Context, request *connect.Request[rtestv1.ListJobsRequest]) (*connect.Response[rtestv1.ListJobsResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	project, err := s.config.Store.AuthorizeProject(ctx, principal, request.Msg.Project)
	if err != nil {
		return nil, connectError(err)
	}
	pageSize := int(request.Msg.PageSize)
	if pageSize == 0 {
		pageSize = int(request.Msg.Limit)
	}
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize < 1 || pageSize > 100 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("page size must be between 1 and 100"))
	}
	page, err := s.config.Store.ListJobs(ctx, project.ID, pageSize, request.Msg.PageToken)
	if err != nil {
		return nil, connectError(err)
	}
	response := &rtestv1.ListJobsResponse{Jobs: make([]*rtestv1.Job, 0, len(page.Jobs)), NextPageToken: page.NextPageToken}
	for _, job := range page.Jobs {
		refreshed, refreshErr := s.refreshJob(ctx, job)
		if refreshErr == nil {
			job = refreshed
		}
		response.Jobs = append(response.Jobs, jobProto(job))
	}
	return connect.NewResponse(response), nil
}

func (s *Server) CancelJob(ctx context.Context, request *connect.Request[rtestv1.CancelJobRequest]) (*connect.Response[rtestv1.CancelJobResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	job, err := s.authorizedJob(ctx, principal, request.Msg.Id)
	if err != nil {
		return nil, err
	}
	if job.Status != protocol.StatusPreparing {
		if err := s.config.Scheduler.Cancel(ctx, job.ID); err != nil {
			return nil, connect.NewError(connect.CodeUnavailable, err)
		}
	}
	job, err = s.config.Store.RequestJobCancellation(ctx, job.ID)
	if err != nil {
		return nil, connectError(err)
	}
	if job.Status != protocol.StatusCancelled {
		job, _ = s.refreshJob(ctx, job)
	}
	if err := s.config.Store.Audit(ctx, principal, job.ProjectID, "job.cancel", job.ID, nil); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&rtestv1.CancelJobResponse{Job: jobProto(job)}), nil
}

func (s *Server) StreamJobLogs(ctx context.Context, request *connect.Request[rtestv1.StreamJobLogsRequest], stream *connect.ServerStream[rtestv1.StreamJobLogsResponse]) error {
	if request.Msg.Offset < 0 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("log offset cannot be negative"))
	}
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return err
	}
	job, err := s.authorizedJob(ctx, principal, request.Msg.Id)
	if err != nil {
		return err
	}
	writer := &streamWriter{stream: stream, offset: request.Msg.Offset}
	if job.Status != protocol.StatusPreparing {
		output := io.Writer(writer)
		if request.Msg.Offset > 0 {
			output = &offsetWriter{remaining: request.Msg.Offset, destination: writer}
		}
		if job.Status.Terminal() {
			if err := s.config.Scheduler.Logs(ctx, job.ID, false, output); err != nil && ctx.Err() == nil {
				return connect.NewError(connect.CodeUnavailable, err)
			}
		} else {
			job, err = s.followJob(ctx, job, output)
			if err != nil {
				return err
			}
		}
	}
	nextOffset := request.Msg.Offset
	if job.Status != protocol.StatusPreparing {
		nextOffset = writer.offset
	}
	return stream.Send(&rtestv1.StreamJobLogsResponse{TerminalJob: jobProto(job), NextOffset: nextOffset})
}

func (s *Server) followJob(ctx context.Context, job control.Job, output io.Writer) (control.Job, error) {
	logsCtx, stopLogs := context.WithCancel(ctx)
	defer stopLogs()
	logsDone := make(chan error, 1)
	jobID := job.ID
	go func(id string, done chan<- error) {
		done <- s.config.Scheduler.Logs(logsCtx, id, true, output)
	}(jobID, logsDone)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		refreshed, err := s.refreshJob(ctx, job)
		if err != nil {
			stopLogs()
			if logsDone != nil {
				<-logsDone
			}
			return control.Job{}, connectError(err)
		}
		job = refreshed
		if job.Status.Terminal() {
			stopLogs()
			if logsDone != nil {
				<-logsDone
			}
			return job, nil
		}
		select {
		case <-ctx.Done():
			stopLogs()
			if logsDone != nil {
				<-logsDone
			}
			return control.Job{}, connect.NewError(connect.CodeCanceled, ctx.Err())
		case err := <-logsDone:
			if err != nil && ctx.Err() == nil {
				return control.Job{}, connect.NewError(connect.CodeUnavailable, err)
			}
			// Docker can close a log stream before the service status is visible as
			// terminal. Continue polling without a live follower in that case.
			logsDone = nil
		case <-ticker.C:
		}
	}
}

func (s *Server) PrepareBuild(ctx context.Context, request *connect.Request[rtestv1.PrepareBuildRequest]) (*connect.Response[rtestv1.PrepareBuildResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	project, err := s.config.Store.AuthorizeProject(ctx, principal, request.Msg.Project)
	if err != nil {
		return nil, connectError(err)
	}
	if !idempotencyKeyPattern.MatchString(request.Msg.IdempotencyKey) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("idempotency key must contain 8 to 128 safe characters"))
	}
	requestHash, err := admissionHash(struct{ ProjectID string }{ProjectID: project.ID})
	if err != nil {
		return nil, connectError(err)
	}
	build, replayed, err := s.config.Store.CreateBuild(ctx, project.ID, control.Idempotency{Key: request.Msg.IdempotencyKey, RequestHash: requestHash})
	if err != nil {
		return nil, connectError(err)
	}
	credential, err := s.config.Authority.Issue(pki.OperationBuild, build.ID, s.config.CredentialTTL)
	if err != nil {
		if !replayed {
			_, _ = s.config.Store.FinishBuild(ctx, build.ID, control.BuildFailed, 1)
		}
		return nil, connectError(err)
	}
	if !replayed {
		if err := s.config.Store.Audit(ctx, principal, project.ID, "build.prepare", build.ID, nil); err != nil {
			return nil, connectError(err)
		}
	}
	return connect.NewResponse(&rtestv1.PrepareBuildResponse{Build: buildProto(build), Buildkit: connectionProto(s.config.BuildKitEndpoint, "", credential)}), nil
}

func (s *Server) FinishBuild(ctx context.Context, request *connect.Request[rtestv1.FinishBuildRequest]) (*connect.Response[rtestv1.FinishBuildResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	build, err := s.config.Store.Build(ctx, request.Msg.Id)
	if err != nil {
		return nil, connectError(err)
	}
	if _, err := s.config.Store.AuthorizeProject(ctx, principal, build.ProjectID); err != nil {
		return nil, connectError(err)
	}
	status := control.BuildSucceeded
	if request.Msg.Cancelled {
		status = control.BuildCancelled
	} else if request.Msg.ExitCode != 0 {
		status = control.BuildFailed
	}
	build, err = s.config.Store.FinishBuild(ctx, build.ID, status, int(request.Msg.ExitCode))
	if err != nil {
		return nil, connectError(err)
	}
	if err := s.config.Store.Audit(ctx, principal, build.ProjectID, "build.finish", build.ID, map[string]string{"status": string(status)}); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&rtestv1.FinishBuildResponse{Build: buildProto(build)}), nil
}

func (s *Server) ExchangeGitHubOIDC(ctx context.Context, request *connect.Request[rtestv1.ExchangeGitHubOIDCRequest]) (*connect.Response[rtestv1.ExchangeGitHubOIDCResponse], error) {
	if s.config.OIDCVerifier == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("GitHub OIDC is not configured"))
	}
	if request.Msg.Project == "" || request.Msg.IdToken == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("project and ID token are required"))
	}
	claims, err := s.config.OIDCVerifier.Verify(ctx, request.Msg.IdToken)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid GitHub identity"))
	}
	trust, err := s.config.Store.MatchGitHubTrust(ctx, request.Msg.Project, claims)
	if err != nil {
		return nil, connectError(err)
	}
	expires := time.Now().UTC().Add(15 * time.Minute)
	if !claims.ExpiresAt.IsZero() && claims.ExpiresAt.Before(expires) {
		expires = claims.ExpiresAt
	}
	issued, err := s.config.Store.CreateProjectSession(ctx, trust.ProjectID, claims.Subject, expires)
	if err != nil {
		return nil, connectError(err)
	}
	principal := control.Principal{Kind: control.PrincipalGitHub, ProjectID: trust.ProjectID, Subject: claims.Subject}
	if err := s.config.Store.Audit(ctx, principal, trust.ProjectID, "github.exchange", trust.ID, map[string]string{"repository_id": claims.RepositoryID}); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&rtestv1.ExchangeGitHubOIDCResponse{Token: issued.Token, ExpiresAt: timestamppb.New(issued.ExpiresAt)}), nil
}

func (s *Server) CreateGitHubTrust(ctx context.Context, request *connect.Request[rtestv1.CreateGitHubTrustRequest]) (*connect.Response[rtestv1.CreateGitHubTrustResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	trust, err := s.config.Store.CreateGitHubTrust(ctx, principal, control.GitHubTrust{
		ProjectID: request.Msg.Project, RepositoryOwnerID: request.Msg.RepositoryOwnerId, RepositoryID: request.Msg.RepositoryId,
		WorkflowRef: request.Msg.WorkflowRef, Ref: request.Msg.Ref, Environment: request.Msg.Environment, Events: append([]string(nil), request.Msg.Events...),
	})
	if err != nil {
		return nil, connectError(err)
	}
	if err := s.config.Store.Audit(ctx, principal, trust.ProjectID, "github.trust.create", trust.ID, nil); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&rtestv1.CreateGitHubTrustResponse{Trust: trustProto(trust)}), nil
}

func (s *Server) ListGitHubTrusts(ctx context.Context, request *connect.Request[rtestv1.ListGitHubTrustsRequest]) (*connect.Response[rtestv1.ListGitHubTrustsResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	trusts, err := s.config.Store.ListGitHubTrusts(ctx, principal, request.Msg.Project)
	if err != nil {
		return nil, connectError(err)
	}
	response := &rtestv1.ListGitHubTrustsResponse{Trusts: make([]*rtestv1.GitHubTrust, 0, len(trusts))}
	for _, trust := range trusts {
		response.Trusts = append(response.Trusts, trustProto(trust))
	}
	return connect.NewResponse(response), nil
}

func (s *Server) RevokeGitHubTrust(ctx context.Context, request *connect.Request[rtestv1.RevokeGitHubTrustRequest]) (*connect.Response[rtestv1.RevokeGitHubTrustResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	if err := s.config.Store.RevokeGitHubTrust(ctx, principal, request.Msg.Id); err != nil {
		return nil, connectError(err)
	}
	if err := s.config.Store.Audit(ctx, principal, "", "github.trust.revoke", request.Msg.Id, nil); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&rtestv1.RevokeGitHubTrustResponse{}), nil
}

func (s *Server) CreateDeviceToken(ctx context.Context, request *connect.Request[rtestv1.CreateDeviceTokenRequest]) (*connect.Response[rtestv1.CreateDeviceTokenResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	var expires time.Time
	if request.Msg.ExpiresAt != nil {
		expires = request.Msg.ExpiresAt.AsTime()
	}
	issued, err := s.config.Store.CreateDeviceToken(ctx, principal, control.CreateDeviceToken{UserID: request.Msg.UserId, Name: request.Msg.Name, ExpiresAt: expires})
	if err != nil {
		return nil, connectError(err)
	}
	if err := s.config.Store.Audit(ctx, principal, "", "device-token.create", issued.Metadata.ID, nil); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&rtestv1.CreateDeviceTokenResponse{TokenMetadata: tokenProto(issued.Metadata), Token: issued.Secret}), nil
}

func (s *Server) ListDeviceTokens(ctx context.Context, request *connect.Request[rtestv1.ListDeviceTokensRequest]) (*connect.Response[rtestv1.ListDeviceTokensResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	tokens, err := s.config.Store.ListDeviceTokens(ctx, principal)
	if err != nil {
		return nil, connectError(err)
	}
	response := &rtestv1.ListDeviceTokensResponse{Tokens: make([]*rtestv1.DeviceToken, 0, len(tokens))}
	for _, token := range tokens {
		response.Tokens = append(response.Tokens, tokenProto(token))
	}
	return connect.NewResponse(response), nil
}

func (s *Server) RevokeDeviceToken(ctx context.Context, request *connect.Request[rtestv1.RevokeDeviceTokenRequest]) (*connect.Response[rtestv1.RevokeDeviceTokenResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	if err := s.config.Store.RevokeDeviceToken(ctx, principal, request.Msg.Id); err != nil {
		return nil, connectError(err)
	}
	if err := s.config.Store.Audit(ctx, principal, "", "device-token.revoke", request.Msg.Id, nil); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&rtestv1.RevokeDeviceTokenResponse{}), nil
}

func (s *Server) CreateEnrollmentCode(ctx context.Context, request *connect.Request[rtestv1.CreateEnrollmentCodeRequest]) (*connect.Response[rtestv1.CreateEnrollmentCodeResponse], error) {
	principal, err := s.authenticate(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	if request.Msg.ExpiresAt == nil || request.Msg.ExpiresAt.CheckValid() != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("valid enrollment expiry is required"))
	}
	if len(request.Msg.DeviceName) < 1 || len(request.Msg.DeviceName) > 128 || strings.ContainsRune(request.Msg.DeviceName, 0) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("device name must contain 1 to 128 characters"))
	}
	now := time.Now().UTC()
	expiresAt := request.Msg.ExpiresAt.AsTime()
	if expiresAt.Before(now.Add(time.Minute)) || expiresAt.After(now.Add(30*time.Minute)) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("enrollment expiry must be between 1 and 30 minutes"))
	}
	issued, err := s.config.Store.CreateEnrollmentCode(ctx, principal, request.Msg.UserId, request.Msg.DeviceName, expiresAt)
	if err != nil {
		return nil, connectError(err)
	}
	if err := s.config.Store.Audit(ctx, principal, "", "enrollment.create", issued.Metadata.ID, map[string]string{"user_id": issued.Metadata.UserID, "device_name": issued.Metadata.DeviceName}); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&rtestv1.CreateEnrollmentCodeResponse{Enrollment: enrollmentProto(issued.Metadata), Code: issued.Secret}), nil
}

func (s *Server) ExchangeEnrollmentCode(ctx context.Context, request *connect.Request[rtestv1.ExchangeEnrollmentCodeRequest]) (*connect.Response[rtestv1.ExchangeEnrollmentCodeResponse], error) {
	if len(request.Msg.Code) < 32 || len(request.Msg.Code) > 256 {
		return nil, connectError(control.ErrUnauthenticated)
	}
	issued, enrollment, err := s.config.Store.ExchangeEnrollmentCode(ctx, request.Msg.Code)
	if err != nil {
		return nil, connectError(err)
	}
	principal := control.Principal{Kind: control.PrincipalDevice, TokenID: issued.Metadata.ID, UserID: issued.Metadata.UserID}
	if err := s.config.Store.Audit(ctx, principal, "", "enrollment.exchange", enrollment.ID, map[string]string{"device_token_id": issued.Metadata.ID}); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&rtestv1.ExchangeEnrollmentCodeResponse{DeviceToken: tokenProto(issued.Metadata), Token: issued.Secret}), nil
}

func (s *Server) authenticate(ctx context.Context, header http.Header) (control.Principal, error) {
	value := header.Get("Authorization")
	token, ok := strings.CutPrefix(value, "Bearer ")
	if !ok || token == "" || strings.ContainsAny(token, " \t\r\n") {
		return control.Principal{}, connect.NewError(connect.CodeUnauthenticated, control.ErrUnauthenticated)
	}
	principal, err := s.config.Store.Authenticate(ctx, token)
	if err != nil {
		return control.Principal{}, connectError(err)
	}
	return principal, nil
}

func (s *Server) authorizedJob(ctx context.Context, principal control.Principal, id string) (control.Job, error) {
	job, err := s.config.Store.Job(ctx, id)
	if err != nil {
		return control.Job{}, connectError(err)
	}
	if _, err := s.config.Store.AuthorizeProject(ctx, principal, job.ProjectID); err != nil {
		return control.Job{}, connectError(err)
	}
	return job, nil
}

func (s *Server) refreshJob(ctx context.Context, job control.Job) (control.Job, error) {
	if job.Status == protocol.StatusPreparing || job.Status.Terminal() {
		return job, nil
	}
	remote, err := s.config.Scheduler.Status(ctx, job.ID)
	if err != nil {
		return control.Job{}, err
	}
	return s.config.Store.SyncJob(ctx, job.ID, remote)
}

var (
	imageDigestPattern    = regexp.MustCompile(`^.+@sha256:[0-9a-f]{64}$`)
	rootDigestPattern     = regexp.MustCompile(`^[0-9a-f]{64}/[1-9][0-9]*$`)
	environmentKey        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)
	cacheNamePattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)
)

func (s *Server) validateJob(projectID string, message *rtestv1.PrepareJobRequest) (control.PrepareJob, error) {
	if message.Image == "" || !s.config.AllowUnpinnedImages && !imageDigestPattern.MatchString(message.Image) {
		return control.PrepareJob{}, errors.New("image must be pinned by sha256 digest")
	}
	if len(message.Command) == 0 || len(message.Command) > 128 {
		return control.PrepareJob{}, errors.New("command must contain between 1 and 128 arguments")
	}
	for _, argument := range message.Command {
		if strings.ContainsRune(argument, 0) {
			return control.PrepareJob{}, errors.New("command contains a NUL byte")
		}
	}
	workingDirectory := message.WorkingDirectory
	if workingDirectory == "" {
		workingDirectory = "."
	}
	clean := filepath.Clean(workingDirectory)
	if filepath.IsAbs(workingDirectory) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return control.PrepareJob{}, errors.New("working directory must remain within the workspace")
	}
	if len(message.Environment) > 128 {
		return control.PrepareJob{}, errors.New("environment may contain at most 128 values")
	}
	caches, err := validateCaches(message.Caches)
	if err != nil {
		return control.PrepareJob{}, err
	}
	for key, value := range message.Environment {
		if !environmentKey.MatchString(key) || strings.ContainsRune(value, 0) {
			return control.PrepareJob{}, fmt.Errorf("invalid environment value %q", key)
		}
	}
	timeout := 30 * time.Minute
	if message.Timeout != nil {
		if err := message.Timeout.CheckValid(); err != nil {
			return control.PrepareJob{}, errors.New("invalid timeout")
		}
		timeout = message.Timeout.AsDuration()
	}
	if timeout < time.Second || timeout > time.Hour {
		return control.PrepareJob{}, errors.New("timeout must be between 1 second and 1 hour")
	}
	if strings.TrimSpace(message.Cpus) == "" || strings.TrimSpace(message.Memory) == "" {
		return control.PrepareJob{}, errors.New("CPU and memory reservations are required")
	}
	if !idempotencyKeyPattern.MatchString(message.IdempotencyKey) {
		return control.PrepareJob{}, errors.New("idempotency key must contain 8 to 128 safe characters")
	}
	return control.PrepareJob{
		ProjectID: projectID, Image: message.Image, Command: append([]string(nil), message.Command...),
		WorkingDirectory: clean, Environment: cloneMap(message.Environment), Timeout: timeout,
		CPUs: message.Cpus, Memory: message.Memory, Caches: caches,
	}, nil
}

func validateCaches(input []*rtestv1.CacheMount) ([]control.CacheMount, error) {
	if len(input) > 16 {
		return nil, errors.New("a job may declare at most 16 caches")
	}
	result := make([]control.CacheMount, 0, len(input))
	names, targets := map[string]bool{}, map[string]bool{}
	reserved := []string{"/var/run/docker.sock", "/usr/local/bin/rtest-job-entrypoint"}
	for _, item := range input {
		if item == nil || !cacheNamePattern.MatchString(item.Name) {
			return nil, errors.New("cache name must contain 1 to 63 lowercase safe characters")
		}
		target := path.Clean(item.Target)
		if !path.IsAbs(item.Target) || target == "/" || strings.ContainsRune(item.Target, 0) {
			return nil, fmt.Errorf("cache %q target must be an absolute container path below /", item.Name)
		}
		if names[item.Name] {
			return nil, fmt.Errorf("cache name %q is declared more than once", item.Name)
		}
		if targets[target] {
			return nil, fmt.Errorf("cache target %q is declared more than once", target)
		}
		for _, protected := range reserved {
			if pathsOverlap(target, protected) {
				return nil, fmt.Errorf("cache target %q overlaps protected runtime path %q", target, protected)
			}
		}
		names[item.Name], targets[target] = true, true
		result = append(result, control.CacheMount{Name: item.Name, Target: target})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func pathsOverlap(first, second string) bool {
	return first == second || strings.HasPrefix(first, second+"/") || strings.HasPrefix(second, first+"/")
}

type streamWriter struct {
	stream *connect.ServerStream[rtestv1.StreamJobLogsResponse]
	offset int64
}

func (w *streamWriter) Write(data []byte) (int, error) {
	copyData := append([]byte(nil), data...)
	w.offset += int64(len(copyData))
	if err := w.stream.Send(&rtestv1.StreamJobLogsResponse{Data: copyData, NextOffset: w.offset}); err != nil {
		w.offset -= int64(len(copyData))
		return 0, err
	}
	return len(data), nil
}

type offsetWriter struct {
	remaining   int64
	destination io.Writer
}

func (w *offsetWriter) Write(data []byte) (int, error) {
	originalLength := len(data)
	if int64(len(data)) <= w.remaining {
		w.remaining -= int64(len(data))
		return originalLength, nil
	}
	data = data[w.remaining:]
	w.remaining = 0
	if _, err := w.destination.Write(data); err != nil {
		return 0, err
	}
	return originalLength, nil
}

func admissionHash(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func connectionProto(endpoint, instance string, credential pki.Credential) *rtestv1.DataPlaneConnection {
	return &rtestv1.DataPlaneConnection{
		Endpoint: endpoint, InstanceName: instance, ServerName: credential.ServerName,
		CaPem: credential.CAPEM, CertificatePem: credential.CertificatePEM, PrivateKeyPem: credential.PrivateKeyPEM,
		ExpiresAt: timestamppb.New(credential.ExpiresAt),
	}
}

func jobProto(job control.Job) *rtestv1.Job {
	result := &rtestv1.Job{
		Id: job.ID, ProjectId: job.ProjectID, Image: job.Image, Command: append([]string(nil), job.Command...),
		WorkingDirectory: job.WorkingDirectory, Environment: cloneMap(job.Environment), RootDigest: job.RootDigest,
		Status: jobStatusProto(job.Status), Timeout: durationpb.New(job.Timeout), Cpus: job.CPUs, Memory: job.Memory,
		CreatedAt: timestamppb.New(job.CreatedAt), ErrorMessage: job.ErrorMessage, CancelRequested: job.CancelRequested, WorkerId: job.WorkerID,
	}
	for _, cache := range job.Caches {
		result.Caches = append(result.Caches, &rtestv1.CacheMount{Name: cache.Name, Target: cache.Target})
	}
	result.StartedAt, result.FinishedAt = timestamp(job.StartedAt), timestamp(job.FinishedAt)
	if job.ExitCode != nil {
		value := int32(*job.ExitCode)
		result.ExitCode = &value
	}
	return result
}

func jobStatusProto(status protocol.Status) rtestv1.JobStatus {
	switch status {
	case protocol.StatusPreparing:
		return rtestv1.JobStatus_JOB_STATUS_PREPARING
	case protocol.StatusQueued:
		return rtestv1.JobStatus_JOB_STATUS_QUEUED
	case protocol.StatusRunning:
		return rtestv1.JobStatus_JOB_STATUS_RUNNING
	case protocol.StatusSucceeded:
		return rtestv1.JobStatus_JOB_STATUS_SUCCEEDED
	case protocol.StatusFailed:
		return rtestv1.JobStatus_JOB_STATUS_FAILED
	case protocol.StatusCancelled:
		return rtestv1.JobStatus_JOB_STATUS_CANCELLED
	case protocol.StatusTimedOut:
		return rtestv1.JobStatus_JOB_STATUS_TIMED_OUT
	case protocol.StatusLost:
		return rtestv1.JobStatus_JOB_STATUS_LOST
	default:
		return rtestv1.JobStatus_JOB_STATUS_UNSPECIFIED
	}
}

func buildProto(build control.Build) *rtestv1.Build {
	result := &rtestv1.Build{Id: build.ID, ProjectId: build.ProjectID, CreatedAt: timestamppb.New(build.CreatedAt), FinishedAt: timestamp(build.FinishedAt)}
	switch build.Status {
	case control.BuildRunning:
		result.Status = rtestv1.BuildStatus_BUILD_STATUS_RUNNING
	case control.BuildSucceeded:
		result.Status = rtestv1.BuildStatus_BUILD_STATUS_SUCCEEDED
	case control.BuildFailed:
		result.Status = rtestv1.BuildStatus_BUILD_STATUS_FAILED
	case control.BuildCancelled:
		result.Status = rtestv1.BuildStatus_BUILD_STATUS_CANCELLED
	}
	if build.ExitCode != nil {
		value := int32(*build.ExitCode)
		result.ExitCode = &value
	}
	return result
}

func userProto(user control.User) *rtestv1.User {
	return &rtestv1.User{Id: user.ID, Name: user.Name, Admin: user.Admin, CreatedAt: timestamppb.New(user.CreatedAt)}
}

func projectProto(project control.Project) *rtestv1.Project {
	return &rtestv1.Project{
		Id: project.ID, Slug: project.Slug, Name: project.Name, CreatedAt: timestamppb.New(project.CreatedAt),
		ActiveImage: project.ActiveImage, PreviousImage: project.PreviousImage, AllowImageOverrides: project.AllowImageOverrides,
	}
}

func projectImageEventProto(event control.ProjectImageEvent) *rtestv1.ProjectImageEvent {
	return &rtestv1.ProjectImageEvent{
		Id: event.ID, ProjectId: event.ProjectID, Action: event.Action, Image: event.Image,
		ReplacedImage: event.ReplacedImage, Actor: event.Actor, CreatedAt: timestamppb.New(event.CreatedAt),
	}
}

func trustProto(trust control.GitHubTrust) *rtestv1.GitHubTrust {
	events := append([]string(nil), trust.Events...)
	sort.Strings(events)
	return &rtestv1.GitHubTrust{
		Id: trust.ID, ProjectId: trust.ProjectID, RepositoryOwnerId: trust.RepositoryOwnerID, RepositoryId: trust.RepositoryID,
		WorkflowRef: trust.WorkflowRef, Ref: trust.Ref, Environment: trust.Environment, Events: events,
		CreatedAt: timestamppb.New(trust.CreatedAt), RevokedAt: timestamp(trust.RevokedAt),
	}
}

func tokenProto(token control.DeviceToken) *rtestv1.DeviceToken {
	return &rtestv1.DeviceToken{
		Id: token.ID, Name: token.Name, UserId: token.UserID, CreatedAt: timestamppb.New(token.CreatedAt),
		ExpiresAt: timestamp(token.ExpiresAt), LastUsedAt: timestamp(token.LastUsedAt), RevokedAt: timestamp(token.RevokedAt),
	}
}

func enrollmentProto(enrollment control.EnrollmentCode) *rtestv1.EnrollmentCode {
	return &rtestv1.EnrollmentCode{
		Id: enrollment.ID, UserId: enrollment.UserID, DeviceName: enrollment.DeviceName,
		CreatedAt: timestamppb.New(enrollment.CreatedAt), ExpiresAt: timestamppb.New(enrollment.ExpiresAt),
		ConsumedAt: timestamp(enrollment.ConsumedAt), FailedAttempts: int32(enrollment.FailedAttempts), MaxAttempts: int32(enrollment.MaxAttempts),
	}
}

func timestamp(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return timestamppb.New(*value)
}

func cloneMap(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func connectError(err error) error {
	switch {
	case errors.Is(err, control.ErrUnauthenticated):
		return connect.NewError(connect.CodeUnauthenticated, control.ErrUnauthenticated)
	case errors.Is(err, control.ErrForbidden):
		return connect.NewError(connect.CodePermissionDenied, control.ErrForbidden)
	case errors.Is(err, control.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, control.ErrNotFound)
	case errors.Is(err, control.ErrAlreadyExists):
		return connect.NewError(connect.CodeAlreadyExists, control.ErrAlreadyExists)
	case errors.Is(err, control.ErrIdempotencyConflict):
		return connect.NewError(connect.CodeAlreadyExists, control.ErrIdempotencyConflict)
	case errors.Is(err, control.ErrInvalidPageToken):
		return connect.NewError(connect.CodeInvalidArgument, control.ErrInvalidPageToken)
	default:
		return connect.NewError(connect.CodeInternal, errors.New("internal rtest error"))
	}
}

var _ rtestv1connect.ControlServiceHandler = (*Server)(nil)
var _ io.Writer = (*streamWriter)(nil)
var _ io.Writer = (*offsetWriter)(nil)
