package controlapi_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/flidai/leapview/rtest/internal/control"
	"github.com/flidai/leapview/rtest/internal/control/controlapi"
	"github.com/flidai/leapview/rtest/internal/control/pki"
	controlsqlite "github.com/flidai/leapview/rtest/internal/control/sqlite"
	rtestv1 "github.com/flidai/leapview/rtest/internal/gen/rtest/v1"
	"github.com/flidai/leapview/rtest/internal/gen/rtest/v1/rtestv1connect"
	"github.com/flidai/leapview/rtest/internal/protocol"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestAuthenticatedGenericJobLifecycle(t *testing.T) {
	fixture := newFixture(t)
	client := fixture.client(fixture.bootstrap.Token)
	ctx := context.Background()
	prepared, err := client.PrepareJob(ctx, connect.NewRequest(&rtestv1.PrepareJobRequest{
		IdempotencyKey: "job-lifecycle-1",
		Project:        fixture.bootstrap.Project.Slug,
		Image:          "ghcr.io/example/ci@sha256:" + strings.Repeat("1", 64),
		Command:        []string{"task", "test"}, WorkingDirectory: "services/api",
		Environment: map[string]string{"CI": "true"}, Timeout: durationpb.New(10 * time.Minute),
		Cpus: "2", Memory: "4g",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Msg.Job.Status != rtestv1.JobStatus_JOB_STATUS_PREPARING || prepared.Msg.Cas == nil || len(prepared.Msg.Cas.CertificatePem) == 0 {
		t.Fatalf("prepared = %#v", prepared.Msg)
	}
	if fixture.scheduler.createdCount() != 0 {
		t.Fatal("job was scheduled before CAS upload completed")
	}

	started, err := client.StartJob(ctx, connect.NewRequest(&rtestv1.StartJobRequest{
		Id: prepared.Msg.Job.Id, RootDigest: strings.Repeat("a", 64) + "/123",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if fixture.scheduler.createdCount() != 1 || started.Msg.Job.Status != rtestv1.JobStatus_JOB_STATUS_QUEUED {
		t.Fatalf("created=%d started=%#v", fixture.scheduler.createdCount(), started.Msg.Job)
	}
	created := fixture.scheduler.created[0]
	if created.Image != prepared.Msg.Job.Image || created.WorkingDirectory != "services/api" || created.Environment["CI"] != "true" {
		t.Fatalf("scheduled job = %#v", created)
	}

	fixture.scheduler.complete(prepared.Msg.Job.Id, protocol.StatusSucceeded, 0)
	got, err := client.GetJob(ctx, connect.NewRequest(&rtestv1.GetJobRequest{Id: prepared.Msg.Job.Id}))
	if err != nil {
		t.Fatal(err)
	}
	if got.Msg.Job.Status != rtestv1.JobStatus_JOB_STATUS_SUCCEEDED || got.Msg.Job.ExitCode == nil || *got.Msg.Job.ExitCode != 0 {
		t.Fatalf("job = %#v", got.Msg.Job)
	}
	stream, err := client.StreamJobLogs(ctx, connect.NewRequest(&rtestv1.StreamJobLogsRequest{Id: prepared.Msg.Job.Id}))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	var terminal *rtestv1.Job
	for stream.Receive() {
		output.Write(stream.Msg().Data)
		if stream.Msg().TerminalJob != nil {
			terminal = stream.Msg().TerminalJob
		}
	}
	if err := stream.Err(); err != nil {
		t.Fatal(err)
	}
	if output.String() != "remote output\n" || terminal == nil || terminal.Status != rtestv1.JobStatus_JOB_STATUS_SUCCEEDED {
		t.Fatalf("output=%q terminal=%#v", output.String(), terminal)
	}
}

func TestProjectImageActivationResolvesDefaultAndRollbackIsAudited(t *testing.T) {
	fixture := newFixture(t)
	client := fixture.client(fixture.bootstrap.Token)
	ctx := context.Background()
	first := "ghcr.io/example/runner@sha256:" + strings.Repeat("a", 64)
	second := "ghcr.io/example/runner@sha256:" + strings.Repeat("b", 64)

	activated, err := client.ActivateProjectImage(ctx, connect.NewRequest(&rtestv1.ActivateProjectImageRequest{
		Project: fixture.bootstrap.Project.Slug, Image: first,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if activated.Msg.Project.ActiveImage != first || !activated.Msg.Project.AllowImageOverrides {
		t.Fatalf("activated project = %#v", activated.Msg.Project)
	}
	prepared, err := client.PrepareJob(ctx, connect.NewRequest(&rtestv1.PrepareJobRequest{
		IdempotencyKey: "default-image-job", Project: fixture.bootstrap.Project.Slug,
		Command: []string{"go", "version"}, Timeout: durationpb.New(time.Minute), Cpus: "1", Memory: "1g",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Msg.Job.Image != first {
		t.Fatalf("resolved image = %q, want %q", prepared.Msg.Job.Image, first)
	}

	if _, err := client.ActivateProjectImage(ctx, connect.NewRequest(&rtestv1.ActivateProjectImageRequest{
		Project: fixture.bootstrap.Project.Slug, Image: second,
	})); err != nil {
		t.Fatal(err)
	}
	rolledBack, err := client.RollbackProjectImage(ctx, connect.NewRequest(&rtestv1.RollbackProjectImageRequest{Project: fixture.bootstrap.Project.Slug}))
	if err != nil {
		t.Fatal(err)
	}
	if rolledBack.Msg.Project.ActiveImage != first || rolledBack.Msg.Project.PreviousImage != second {
		t.Fatalf("rolled back project = %#v", rolledBack.Msg.Project)
	}
	history, err := client.ListProjectImageHistory(ctx, connect.NewRequest(&rtestv1.ListProjectImageHistoryRequest{Project: fixture.bootstrap.Project.Slug}))
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Msg.Events) != 3 || history.Msg.Events[0].Action != "rollback" || history.Msg.Events[0].Image != first {
		t.Fatalf("history = %#v", history.Msg.Events)
	}
}

func TestProjectImageActivationFailsClosedBeforeMutation(t *testing.T) {
	fixture := newFixture(t)
	client := fixture.client(fixture.bootstrap.Token)
	ctx := context.Background()
	good := "ghcr.io/example/runner@sha256:" + strings.Repeat("c", 64)
	bad := "ghcr.io/example/runner@sha256:" + strings.Repeat("d", 64)
	if _, err := client.ActivateProjectImage(ctx, connect.NewRequest(&rtestv1.ActivateProjectImageRequest{Project: "example", Image: good})); err != nil {
		t.Fatal(err)
	}
	fixture.scheduler.validateErr = errors.New("registry unavailable")
	if _, err := client.ActivateProjectImage(ctx, connect.NewRequest(&rtestv1.ActivateProjectImageRequest{Project: "example", Image: bad})); connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("activation error = %v", err)
	}
	projects, err := client.ListProjects(ctx, connect.NewRequest(&rtestv1.ListProjectsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if projects.Msg.Projects[0].ActiveImage != good || projects.Msg.Projects[0].PreviousImage != "" {
		t.Fatalf("failed activation mutated project = %#v", projects.Msg.Projects[0])
	}
}

func TestProjectImageOverridePolicyIsEnforcedAtAdmission(t *testing.T) {
	fixture := newFixture(t)
	client := fixture.client(fixture.bootstrap.Token)
	ctx := context.Background()
	active := "ghcr.io/example/runner@sha256:" + strings.Repeat("e", 64)
	override := "ghcr.io/example/runner@sha256:" + strings.Repeat("f", 64)
	if _, err := client.ActivateProjectImage(ctx, connect.NewRequest(&rtestv1.ActivateProjectImageRequest{Project: "example", Image: active})); err != nil {
		t.Fatal(err)
	}
	if _, err := client.SetProjectImagePolicy(ctx, connect.NewRequest(&rtestv1.SetProjectImagePolicyRequest{Project: "example", AllowImageOverrides: false})); err != nil {
		t.Fatal(err)
	}
	_, err := client.PrepareJob(ctx, connect.NewRequest(&rtestv1.PrepareJobRequest{
		IdempotencyKey: "denied-override-job", Project: "example", Image: override,
		Command: []string{"true"}, Timeout: durationpb.New(time.Minute), Cpus: "1", Memory: "1g",
	}))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("override error = %v", err)
	}
}

func TestStreamingLogsStopsFollowerWhenJobBecomesTerminal(t *testing.T) {
	fixture := newFixture(t)
	fixture.scheduler.blockFollowingLogs = true
	fixture.scheduler.logsStarted = make(chan struct{})
	client := fixture.client(fixture.bootstrap.Token)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	prepared, err := client.PrepareJob(ctx, connect.NewRequest(&rtestv1.PrepareJobRequest{
		IdempotencyKey: "job-follow-1",
		Project:        fixture.bootstrap.Project.Slug,
		Image:          "ghcr.io/example/ci@sha256:" + strings.Repeat("3", 64),
		Command:        []string{"true"}, Timeout: durationpb.New(time.Minute), Cpus: "1", Memory: "1g",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.StartJob(ctx, connect.NewRequest(&rtestv1.StartJobRequest{Id: prepared.Msg.Job.Id, RootDigest: strings.Repeat("b", 64) + "/1"})); err != nil {
		t.Fatal(err)
	}
	stream, err := client.StreamJobLogs(ctx, connect.NewRequest(&rtestv1.StreamJobLogsRequest{Id: prepared.Msg.Job.Id}))
	if err != nil {
		t.Fatal(err)
	}
	terminal := make(chan *rtestv1.Job, 1)
	streamError := make(chan error, 1)
	go func() {
		var job *rtestv1.Job
		for stream.Receive() {
			if stream.Msg().TerminalJob != nil {
				job = stream.Msg().TerminalJob
			}
		}
		terminal <- job
		streamError <- stream.Err()
	}()
	select {
	case <-fixture.scheduler.logsStarted:
	case <-ctx.Done():
		t.Fatal("log follower did not start")
	}
	fixture.scheduler.complete(prepared.Msg.Job.Id, protocol.StatusSucceeded, 0)
	if err := <-streamError; err != nil {
		t.Fatal(err)
	}
	if job := <-terminal; job == nil || job.Status != rtestv1.JobStatus_JOB_STATUS_SUCCEEDED {
		t.Fatalf("terminal job = %#v", job)
	}
}

func TestProjectAuthorizationPreventsCrossProjectJobAccess(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	owner, _ := fixture.store.Authenticate(ctx, fixture.bootstrap.Token)
	other, err := fixture.store.CreateProject(ctx, owner, "other", "Other")
	if err != nil {
		t.Fatal(err)
	}
	member, err := fixture.store.CreateUser(ctx, owner, "Member", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.store.AddProjectMember(ctx, owner, other.ID, member.ID); err != nil {
		t.Fatal(err)
	}
	issued, err := fixture.store.CreateDeviceToken(ctx, owner, control.CreateDeviceToken{UserID: member.ID, Name: "member"})
	if err != nil {
		t.Fatal(err)
	}

	ownerClient := fixture.client(fixture.bootstrap.Token)
	prepared, err := ownerClient.PrepareJob(ctx, connect.NewRequest(&rtestv1.PrepareJobRequest{
		IdempotencyKey: "job-authorization-1",
		Project:        fixture.bootstrap.Project.ID, Image: "ghcr.io/example/ci@sha256:" + strings.Repeat("2", 64),
		Command: []string{"true"}, Timeout: durationpb.New(time.Minute), Cpus: "1", Memory: "1g",
	}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = fixture.client(issued.Secret).GetJob(ctx, connect.NewRequest(&rtestv1.GetJobRequest{Id: prepared.Msg.Job.Id}))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("cross-project error = %v", err)
	}
	_, err = fixture.client(issued.Secret).CancelJob(ctx, connect.NewRequest(&rtestv1.CancelJobRequest{Id: prepared.Msg.Job.Id}))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("cross-project cancel error = %v", err)
	}
	stream, err := fixture.client(issued.Secret).StreamJobLogs(ctx, connect.NewRequest(&rtestv1.StreamJobLogsRequest{Id: prepared.Msg.Job.Id}))
	if err == nil {
		for stream.Receive() {
		}
		err = stream.Err()
	}
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("cross-project stream error = %v", err)
	}
	_, err = fixture.client("").ListJobs(ctx, connect.NewRequest(&rtestv1.ListJobsRequest{Project: other.ID}))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("unauthenticated error = %v", err)
	}
}

func TestBuildPreparationReturnsBuildScopedCertificate(t *testing.T) {
	fixture := newFixture(t)
	response, err := fixture.client(fixture.bootstrap.Token).PrepareBuild(context.Background(), connect.NewRequest(&rtestv1.PrepareBuildRequest{
		Project: fixture.bootstrap.Project.ID, IdempotencyKey: "build-certificate-1",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.Buildkit.Endpoint != "buildkit.example:1234" || len(response.Msg.Buildkit.PrivateKeyPem) == 0 {
		t.Fatalf("response = %#v", response.Msg)
	}
	if fixture.store.OperationActive(context.Background(), "job", response.Msg.Build.Id) {
		t.Fatal("build certificate was accepted as job credential")
	}
	if !fixture.store.OperationActive(context.Background(), "build", response.Msg.Build.Id) {
		t.Fatal("build credential is not active")
	}
	finished, err := fixture.client(fixture.bootstrap.Token).FinishBuild(context.Background(), connect.NewRequest(&rtestv1.FinishBuildRequest{Id: response.Msg.Build.Id, ExitCode: 0}))
	if err != nil || finished.Msg.Build.Status != rtestv1.BuildStatus_BUILD_STATUS_SUCCEEDED {
		t.Fatalf("finished=%#v err=%v", finished, err)
	}
	if fixture.store.OperationActive(context.Background(), "build", response.Msg.Build.Id) {
		t.Fatal("finished build credential remains active")
	}
}

func TestAdmissionIdempotencyReplaysResourcesAndRejectsChangedRequests(t *testing.T) {
	fixture := newFixture(t)
	client := fixture.client(fixture.bootstrap.Token)
	ctx := context.Background()
	request := &rtestv1.PrepareJobRequest{
		Project: fixture.bootstrap.Project.ID, IdempotencyKey: "job-retry-1",
		Image: "ghcr.io/example/ci@sha256:" + strings.Repeat("4", 64), Command: []string{"task", "test"},
		Timeout: durationpb.New(time.Minute), Cpus: "1", Memory: "1g",
	}
	first, err := client.PrepareJob(ctx, connect.NewRequest(request))
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.PrepareJob(ctx, connect.NewRequest(request))
	if err != nil {
		t.Fatal(err)
	}
	if second.Msg.Job.Id != first.Msg.Job.Id {
		t.Fatalf("replayed job ID = %q, want %q", second.Msg.Job.Id, first.Msg.Job.Id)
	}
	changed := *request
	changed.Command = []string{"task", "ci"}
	if _, err := client.PrepareJob(ctx, connect.NewRequest(&changed)); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("changed idempotent request error = %v", err)
	}

	buildRequest := &rtestv1.PrepareBuildRequest{Project: fixture.bootstrap.Project.ID, IdempotencyKey: "build-retry-1"}
	firstBuild, err := client.PrepareBuild(ctx, connect.NewRequest(buildRequest))
	if err != nil {
		t.Fatal(err)
	}
	secondBuild, err := client.PrepareBuild(ctx, connect.NewRequest(buildRequest))
	if err != nil {
		t.Fatal(err)
	}
	if secondBuild.Msg.Build.Id != firstBuild.Msg.Build.Id {
		t.Fatalf("replayed build ID = %q, want %q", secondBuild.Msg.Build.Id, firstBuild.Msg.Build.Id)
	}
}

func TestOneDeviceCredentialListsEveryAuthorizedProject(t *testing.T) {
	fixture := newFixture(t)
	ctx := context.Background()
	ownerClient := fixture.client(fixture.bootstrap.Token)
	second, err := ownerClient.CreateProject(ctx, connect.NewRequest(&rtestv1.CreateProjectRequest{Slug: "second-project", Name: "Second project"}))
	if err != nil {
		t.Fatal(err)
	}
	member, err := ownerClient.CreateUser(ctx, connect.NewRequest(&rtestv1.CreateUserRequest{Name: "Project member"}))
	if err != nil {
		t.Fatal(err)
	}
	for _, project := range []string{fixture.bootstrap.Project.ID, second.Msg.Project.Id} {
		if _, err := ownerClient.AddProjectMember(ctx, connect.NewRequest(&rtestv1.AddProjectMemberRequest{Project: project, UserId: member.Msg.User.Id})); err != nil {
			t.Fatal(err)
		}
	}
	issued, err := ownerClient.CreateDeviceToken(ctx, connect.NewRequest(&rtestv1.CreateDeviceTokenRequest{Name: "member-laptop", UserId: member.Msg.User.Id}))
	if err != nil {
		t.Fatal(err)
	}
	projects, err := fixture.client(issued.Msg.Token).ListProjects(ctx, connect.NewRequest(&rtestv1.ListProjectsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(projects.Msg.Projects) != 2 || projects.Msg.Projects[0].Slug != "example" || projects.Msg.Projects[1].Slug != "second-project" {
		t.Fatalf("projects = %#v", projects.Msg.Projects)
	}
}

func TestListJobsUsesOpaqueProjectBoundKeysetPagination(t *testing.T) {
	fixture := newFixture(t)
	client := fixture.client(fixture.bootstrap.Token)
	ctx := context.Background()
	for index := 0; index < 3; index++ {
		_, err := client.PrepareJob(ctx, connect.NewRequest(&rtestv1.PrepareJobRequest{
			Project: fixture.bootstrap.Project.ID, IdempotencyKey: fmt.Sprintf("page-job-%d", index),
			Image: "ghcr.io/example/ci@sha256:" + strings.Repeat("5", 64), Command: []string{"true"},
			Timeout: durationpb.New(time.Minute), Cpus: "1", Memory: "1g",
		}))
		if err != nil {
			t.Fatal(err)
		}
	}
	first, err := client.ListJobs(ctx, connect.NewRequest(&rtestv1.ListJobsRequest{Project: fixture.bootstrap.Project.ID, PageSize: 2}))
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Msg.Jobs) != 2 || first.Msg.NextPageToken == "" {
		t.Fatalf("first page = %#v", first.Msg)
	}
	second, err := client.ListJobs(ctx, connect.NewRequest(&rtestv1.ListJobsRequest{
		Project: fixture.bootstrap.Project.ID, PageSize: 2, PageToken: first.Msg.NextPageToken,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Msg.Jobs) != 1 || second.Msg.NextPageToken != "" || second.Msg.Jobs[0].Id == first.Msg.Jobs[0].Id || second.Msg.Jobs[0].Id == first.Msg.Jobs[1].Id {
		t.Fatalf("second page = %#v", second.Msg)
	}
	if _, err := client.ListJobs(ctx, connect.NewRequest(&rtestv1.ListJobsRequest{
		Project: fixture.bootstrap.Project.ID, PageSize: 2, PageToken: first.Msg.NextPageToken + "tampered",
	})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("tampered page token error = %v", err)
	}
	owner, err := fixture.store.Authenticate(ctx, fixture.bootstrap.Token)
	if err != nil {
		t.Fatal(err)
	}
	otherProject, err := fixture.store.CreateProject(ctx, owner, "other-page-project", "Other page project")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListJobs(ctx, connect.NewRequest(&rtestv1.ListJobsRequest{
		Project: otherProject.ID, PageSize: 2, PageToken: first.Msg.NextPageToken,
	})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("cross-project page token error = %v", err)
	}
}

func TestStreamJobLogsResumesAtByteOffset(t *testing.T) {
	fixture := newFixture(t)
	client := fixture.client(fixture.bootstrap.Token)
	ctx := context.Background()
	prepared, err := client.PrepareJob(ctx, connect.NewRequest(&rtestv1.PrepareJobRequest{
		Project: fixture.bootstrap.Project.ID, IdempotencyKey: "job-log-offset-1",
		Image: "ghcr.io/example/ci@sha256:" + strings.Repeat("6", 64), Command: []string{"true"},
		Timeout: durationpb.New(time.Minute), Cpus: "1", Memory: "1g",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.StartJob(ctx, connect.NewRequest(&rtestv1.StartJobRequest{Id: prepared.Msg.Job.Id, RootDigest: strings.Repeat("c", 64) + "/1"})); err != nil {
		t.Fatal(err)
	}
	fixture.scheduler.complete(prepared.Msg.Job.Id, protocol.StatusSucceeded, 0)
	stream, err := client.StreamJobLogs(ctx, connect.NewRequest(&rtestv1.StreamJobLogsRequest{Id: prepared.Msg.Job.Id, Offset: 7}))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	var nextOffset int64
	for stream.Receive() {
		output.Write(stream.Msg().Data)
		nextOffset = stream.Msg().NextOffset
	}
	if err := stream.Err(); err != nil {
		t.Fatal(err)
	}
	if output.String() != "output\n" || nextOffset != int64(len("remote output\n")) {
		t.Fatalf("output=%q next offset=%d", output.String(), nextOffset)
	}
	stream, err = client.StreamJobLogs(ctx, connect.NewRequest(&rtestv1.StreamJobLogsRequest{Id: prepared.Msg.Job.Id, Offset: -1}))
	if err == nil {
		for stream.Receive() {
		}
		err = stream.Err()
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("negative offset error = %v", err)
	}
}

func TestGitHubOIDCExchangeReturnsProjectScopedTemporaryToken(t *testing.T) {
	claims := control.GitHubClaims{
		Subject: "repo:flidai/leapview:environment:rtest", RepositoryOwnerID: "100", RepositoryID: "200",
		WorkflowRef: "flidai/leapview/.github/workflows/ci.yml@refs/heads/main", Ref: "refs/heads/main",
		Environment: "rtest", EventName: "workflow_dispatch", ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	fixture := newFixtureWithVerifier(t, fakeVerifier{claims: claims})
	client := fixture.client(fixture.bootstrap.Token)
	_, err := client.CreateGitHubTrust(context.Background(), connect.NewRequest(&rtestv1.CreateGitHubTrustRequest{
		Project: fixture.bootstrap.Project.ID, RepositoryOwnerId: "100", RepositoryId: "200",
		WorkflowRef: "flidai/leapview/.github/workflows/ci.yml@refs/heads/*", Ref: "refs/heads/*",
		Environment: "rtest", Events: []string{"workflow_dispatch"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	exchanged, err := fixture.client("").ExchangeGitHubOIDC(context.Background(), connect.NewRequest(&rtestv1.ExchangeGitHubOIDCRequest{
		Project: fixture.bootstrap.Project.ID, IdToken: "signed-github-token",
	}))
	if err != nil {
		t.Fatal(err)
	}
	principal, err := fixture.store.Authenticate(context.Background(), exchanged.Msg.Token)
	if err != nil {
		t.Fatal(err)
	}
	if principal.Kind != control.PrincipalGitHub || principal.ProjectID != fixture.bootstrap.Project.ID || principal.Subject != claims.Subject {
		t.Fatalf("principal = %#v", principal)
	}
	if exchanged.Msg.ExpiresAt.AsTime().After(claims.ExpiresAt.Add(time.Second)) {
		t.Fatalf("exchange expiry %s exceeds identity expiry %s", exchanged.Msg.ExpiresAt.AsTime(), claims.ExpiresAt)
	}
}

type fixture struct {
	store     *controlsqlite.Store
	bootstrap control.BootstrapResult
	scheduler *fakeScheduler
	server    *httptest.Server
}

func newFixture(t *testing.T) *fixture {
	return newFixtureWithVerifier(t, nil)
}

func newFixtureWithVerifier(t *testing.T, verifier controlapi.OIDCVerifier) *fixture {
	t.Helper()
	root := t.TempDir()
	store, err := controlsqlite.Open(filepath.Join(root, "state"), []byte("test-pepper-that-is-at-least-32-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	bootstrap, err := store.Bootstrap(context.Background(), control.Bootstrap{UserName: "Owner", ProjectSlug: "example", ProjectName: "Example", TokenName: "test"})
	if err != nil {
		t.Fatal(err)
	}
	authority, err := pki.Ensure(filepath.Join(root, "pki"), []string{"localhost", "127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	scheduler := &fakeScheduler{jobs: map[string]protocol.Job{}}
	handler, err := controlapi.New(controlapi.Config{
		Store: store, Scheduler: scheduler, Authority: authority,
		CASEndpoint: "cas.example:50051", CASInstance: "rtest", BuildKitEndpoint: "buildkit.example:1234",
		CredentialTTL: 15 * time.Minute, OIDCVerifier: verifier,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &fixture{store: store, bootstrap: bootstrap, scheduler: scheduler, server: server}
}

func (f *fixture) client(token string) rtestv1connect.ControlServiceClient {
	transport := roundTripper(func(request *http.Request) (*http.Response, error) {
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		return http.DefaultTransport.RoundTrip(request)
	})
	return rtestv1connect.NewControlServiceClient(&http.Client{Transport: transport}, f.server.URL)
}

type roundTripper func(*http.Request) (*http.Response, error)

func (r roundTripper) RoundTrip(request *http.Request) (*http.Response, error) { return r(request) }

type fakeScheduler struct {
	mu                 sync.Mutex
	created            []control.Job
	jobs               map[string]protocol.Job
	blockFollowingLogs bool
	logsStarted        chan struct{}
	validateErr        error
}

func (f *fakeScheduler) Check(context.Context) error { return nil }

func (f *fakeScheduler) ValidateImage(context.Context, string) error { return f.validateErr }

func (f *fakeScheduler) Create(_ context.Context, job control.Job) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, job)
	f.jobs[job.ID] = protocol.Job{ID: job.ID, Repository: job.ProjectID, Command: job.Command, Status: protocol.StatusQueued, CreatedAt: job.CreatedAt, TimeoutSeconds: int(job.Timeout.Seconds())}
	return nil
}

func (f *fakeScheduler) Status(_ context.Context, id string) (protocol.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.jobs[id], nil
}

func (f *fakeScheduler) Logs(ctx context.Context, _ string, follow bool, output io.Writer) error {
	if _, err := io.WriteString(output, "remote output\n"); err != nil {
		return err
	}
	if follow && f.blockFollowingLogs {
		close(f.logsStarted)
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

func (f *fakeScheduler) Cancel(_ context.Context, id string) error {
	f.complete(id, protocol.StatusCancelled, 130)
	return nil
}

func (f *fakeScheduler) createdCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.created)
}

func (f *fakeScheduler) complete(id string, status protocol.Status, exitCode int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	job := f.jobs[id]
	now := time.Now().UTC()
	job.Status, job.ExitCode, job.FinishedAt = status, &exitCode, &now
	f.jobs[id] = job
}

var _ control.Scheduler = (*fakeScheduler)(nil)

type fakeVerifier struct {
	claims control.GitHubClaims
	err    error
}

func (f fakeVerifier) Verify(context.Context, string) (control.GitHubClaims, error) {
	return f.claims, f.err
}
