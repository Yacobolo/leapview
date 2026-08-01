package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/flidai/leapview/rtest/internal/config"
	rtestv1 "github.com/flidai/leapview/rtest/internal/gen/rtest/v1"
	"github.com/flidai/leapview/rtest/internal/gen/rtest/v1/rtestv1connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestParseExecUsesGenericProjectImageAndArbitraryArgv(t *testing.T) {
	settings := config.Config{Service: &config.Service{Project: "example", Image: "image@sha256:digest", CPUs: "2", Memory: "4g"}}
	got, err := parseExec(settings, "example", []string{"--timeout", "5m", "--workdir", "service", "--env", "CI=true", "--", "task", "test", "--race"})
	if err != nil {
		t.Fatal(err)
	}
	if got.project != "example" || got.image != "image@sha256:digest" || got.timeout != 5*time.Minute || got.workdir != "service" || got.environment["CI"] != "true" {
		t.Fatalf("options = %#v", got)
	}
	if !reflect.DeepEqual(got.command, []string{"task", "test", "--race"}) {
		t.Fatalf("command = %#v", got.command)
	}
}

func TestParseExecRequiresExplicitCommandBoundary(t *testing.T) {
	settings := config.Config{Service: &config.Service{Project: "example", Image: "image", CPUs: "2", Memory: "4g"}}
	if _, err := parseExec(settings, "example", []string{"go", "test", "./..."}); err == nil {
		t.Fatal("exec accepted a command without -- boundary")
	}
}

func TestParseExecAllowsServerOwnedDefaultImage(t *testing.T) {
	settings := config.Config{Service: &config.Service{CPUs: "2", Memory: "4g"}}
	got, err := parseExec(settings, "example", []string{"--", "go", "version"})
	if err != nil {
		t.Fatal(err)
	}
	if got.project != "example" || got.image != "" || !reflect.DeepEqual(got.command, []string{"go", "version"}) {
		t.Fatalf("options = %#v", got)
	}
}

func TestDefaultExecWorkingDirectoryFollowsInvocationDirectory(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "services", "api")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := defaultExecWorkingDirectory(root, nested)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join("services", "api") {
		t.Fatalf("working directory = %q", got)
	}
	if got, err := defaultExecWorkingDirectory(root, root); err != nil || got != "." {
		t.Fatalf("root working directory = %q, err = %v", got, err)
	}
}

func TestImageActivateUsesRepositoryProjectAndPinnedImage(t *testing.T) {
	service := &projectListService{projects: []*rtestv1.Project{{Id: "prj1", Slug: "example", Name: "Example"}}}
	client, closeServer := testServiceClient(t, service)
	defer closeServer()
	image := "ghcr.io/example/runner@sha256:" + strings.Repeat("a", 64)
	var stdout, stderr bytes.Buffer
	code := serviceImage(context.Background(), client, config.Config{}, "example", []string{"activate", "--image", image}, IO{Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	service.mu.Lock()
	gotProject, gotImage := service.activatedProject, service.activatedImage
	service.mu.Unlock()
	if gotProject != "example" || gotImage != image || !strings.Contains(stdout.String(), image) {
		t.Fatalf("project=%q image=%q stdout=%q", gotProject, gotImage, stdout.String())
	}
}

func TestGlobalTokenIsRemovedBeforeCommandDispatch(t *testing.T) {
	token, args, err := globalArgs([]string{"--token", "secret", "exec", "--", "true"})
	if err != nil || token != "secret" || !reflect.DeepEqual(args, []string{"exec", "--", "true"}) {
		t.Fatalf("token=%q args=%#v err=%v", token, args, err)
	}
}

func TestExplicitProjectRejectsConflictsAndStopsAtCommandBoundary(t *testing.T) {
	project, err := explicitProject([]string{"--image", "example", "--project", "one", "--", "tool", "--project", "ignored"})
	if err != nil || project != "one" {
		t.Fatalf("project=%q err=%v", project, err)
	}
	if _, err := explicitProject([]string{"--project", "one", "--project", "two", "--", "true"}); err == nil {
		t.Fatal("conflicting project flags were accepted")
	}
}

func TestImageHelpersPreserveBuildxArgumentsAndNormalizeTags(t *testing.T) {
	project, args, err := consumeProject("", []string{"--project", "example", "--", "--label", "value=--project"})
	if err != nil || project != "example" || !reflect.DeepEqual(args, []string{"--", "--label", "value=--project"}) {
		t.Fatalf("project=%q args=%#v err=%v", project, args, err)
	}
	for input, want := range map[string]string{
		"ghcr.io/acme/runner:latest": "ghcr.io/acme/runner",
		"localhost:5000/runner:dev":  "localhost:5000/runner",
		"localhost:5000/runner":      "localhost:5000/runner",
	} {
		if got := repositoryFromTag(input); got != want {
			t.Fatalf("repositoryFromTag(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestServiceLoginExchangesEnrollmentFromStdinWithoutEchoingSecrets(t *testing.T) {
	code := "rtest_enr_tok123_" + strings.Repeat("a", 43)
	token := "rtest_dt_tok456_" + strings.Repeat("b", 43)
	service := &enrollmentService{wantCode: code, token: token}
	path, handler := rtestv1connect.NewControlServiceHandler(service)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	defer server.Close()
	keyring := &memoryKeyring{}
	var stdout, stderr bytes.Buffer
	settings := config.Config{URL: server.URL, Service: &config.Service{}}
	result := serviceLogin(context.Background(), settings, "", nil, IO{
		Stdin: strings.NewReader(code + "\n"), Stdout: &stdout, Stderr: &stderr, Keyring: keyring,
	})
	if result != 0 {
		t.Fatalf("result=%d stderr=%q", result, stderr.String())
	}
	if keyring.token != token || service.gotCode != code {
		t.Fatalf("stored=%q exchanged=%q", keyring.token, service.gotCode)
	}
	if strings.Contains(stdout.String()+stderr.String(), code) || strings.Contains(stdout.String()+stderr.String(), token) {
		t.Fatalf("secret was echoed: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if result := serviceLogout(settings, IO{Stdout: &stdout, Stderr: &stderr, Keyring: keyring}); result != 0 || keyring.token != "" {
		t.Fatalf("logout result=%d stored=%q", result, keyring.token)
	}
}

func TestServiceInitWritesOnlyAnAuthorizedProjectLink(t *testing.T) {
	service := &projectListService{projects: []*rtestv1.Project{
		{Id: "prj1", Slug: "one", Name: "One"}, {Id: "prj2", Slug: "two", Name: "Two"},
	}}
	client, closeServer := testServiceClient(t, service)
	defer closeServer()
	directory := t.TempDir()
	initGitRepository(t, directory)
	var stdout, stderr bytes.Buffer
	code := serviceInit(context.Background(), client, []string{"--project", "two"}, IO{Dir: directory, Stdout: &stdout, Stderr: &stderr})
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(directory, "rtest.json"))
	if err != nil || string(data) != "{\n  \"project\": \"two\"\n}\n" {
		t.Fatalf("link=%q err=%v", data, err)
	}

	other := t.TempDir()
	initGitRepository(t, other)
	stdout.Reset()
	stderr.Reset()
	code = serviceInit(context.Background(), client, []string{"--project", "unauthorized"}, IO{Dir: other, Stdout: &stdout, Stderr: &stderr})
	if code == 0 || !strings.Contains(stderr.String(), "not authorized") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(other, "rtest.json")); !os.IsNotExist(err) {
		t.Fatalf("unauthorized init wrote a link: %v", err)
	}
}

func TestServiceExecRejectsUnauthorizedProjectBeforeWorkspaceOrAdmission(t *testing.T) {
	service := &projectListService{projects: []*rtestv1.Project{{Id: "prj1", Slug: "authorized", Name: "Authorized"}}}
	client, closeServer := testServiceClient(t, service)
	defer closeServer()
	settings := config.Config{Service: &config.Service{
		Image: "ghcr.io/example/ci@sha256:" + strings.Repeat("a", 64), CPUs: "1", Memory: "1g",
	}}
	var stdout, stderr bytes.Buffer
	code := serviceExec(context.Background(), client, settings, "unauthorized", []string{"--", "true"}, IO{
		Dir: filepath.Join(t.TempDir(), "not-a-worktree"), Stdout: &stdout, Stderr: &stderr,
	})
	if code == 0 || !strings.Contains(stderr.String(), "not authorized") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	service.mu.Lock()
	prepareCount := service.prepareCount
	service.mu.Unlock()
	if prepareCount != 0 {
		t.Fatalf("PrepareJob called %d times", prepareCount)
	}
}

func TestServiceDoctorUsesEnvironmentProjectForGitHubOIDC(t *testing.T) {
	oidc := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("audience") != "https://rtest.example" || request.Header.Get("Authorization") != "Bearer request-token" {
			t.Errorf("OIDC request audience=%q authorization=%q", request.URL.Query().Get("audience"), request.Header.Get("Authorization"))
		}
		_, _ = response.Write([]byte(`{"value":"github-id-token"}`))
	}))
	defer oidc.Close()
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", oidc.URL)
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "request-token")
	t.Setenv("RTEST_PROJECT", "poc")
	t.Setenv("RTEST_TOKEN", "")

	service := &oidcDoctorService{}
	path, handler := rtestv1connect.NewControlServiceHandler(service)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	defer server.Close()
	var stdout, stderr bytes.Buffer
	code := runService(context.Background(), config.Config{
		URL: server.URL, Service: &config.Service{OIDCAudience: "https://rtest.example"},
	}, "", []string{"doctor"}, IO{Stdout: &stdout, Stderr: &stderr, Keyring: &memoryKeyring{}})
	if code != 0 || service.project != "poc" {
		t.Fatalf("code=%d project=%q stdout=%q stderr=%q", code, service.project, stdout.String(), stderr.String())
	}
}

func initGitRepository(t *testing.T, directory string) {
	t.Helper()
	command := exec.Command("git", "init", "-q")
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
}

func TestWaitServiceJobReconnectsWithoutDuplicatingLogBytes(t *testing.T) {
	service := &interruptedLogService{}
	path, handler := rtestv1connect.NewControlServiceHandler(service)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	defer server.Close()
	client := rtestv1connect.NewControlServiceClient(server.Client(), server.URL)
	var stdout, stderr bytes.Buffer
	code := waitServiceJob(context.Background(), client, "job-1", IO{Stdout: &stdout, Stderr: &stderr})
	if code != 0 || stdout.String() != "abcdef" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	service.mu.Lock()
	offsets := append([]int64(nil), service.offsets...)
	service.mu.Unlock()
	if !reflect.DeepEqual(offsets, []int64{0, 3}) {
		t.Fatalf("stream offsets = %v", offsets)
	}
}

type interruptedLogService struct {
	rtestv1connect.UnimplementedControlServiceHandler
	mu      sync.Mutex
	offsets []int64
}

type enrollmentService struct {
	rtestv1connect.UnimplementedControlServiceHandler
	wantCode string
	gotCode  string
	token    string
}

func (s *enrollmentService) ExchangeEnrollmentCode(_ context.Context, request *connect.Request[rtestv1.ExchangeEnrollmentCodeRequest]) (*connect.Response[rtestv1.ExchangeEnrollmentCodeResponse], error) {
	s.gotCode = request.Msg.Code
	if request.Msg.Code != s.wantCode {
		return nil, connect.NewError(connect.CodeUnauthenticated, context.Canceled)
	}
	return connect.NewResponse(&rtestv1.ExchangeEnrollmentCodeResponse{
		Token: s.token, DeviceToken: &rtestv1.DeviceToken{Id: "tok456", UserId: "usr1", Name: "laptop"},
	}), nil
}

func (s *enrollmentService) ListDeviceTokens(context.Context, *connect.Request[rtestv1.ListDeviceTokensRequest]) (*connect.Response[rtestv1.ListDeviceTokensResponse], error) {
	return connect.NewResponse(&rtestv1.ListDeviceTokensResponse{}), nil
}

type memoryKeyring struct{ token string }

func (m *memoryKeyring) Get(string, string) (string, error) { return m.token, nil }
func (m *memoryKeyring) Set(_, _, token string) error       { m.token = token; return nil }
func (m *memoryKeyring) Delete(string, string) error        { m.token = ""; return nil }

type projectListService struct {
	rtestv1connect.UnimplementedControlServiceHandler
	mu               sync.Mutex
	projects         []*rtestv1.Project
	prepareCount     int
	activatedProject string
	activatedImage   string
}

type oidcDoctorService struct {
	rtestv1connect.UnimplementedControlServiceHandler
	project string
}

func (s *oidcDoctorService) ExchangeGitHubOIDC(_ context.Context, request *connect.Request[rtestv1.ExchangeGitHubOIDCRequest]) (*connect.Response[rtestv1.ExchangeGitHubOIDCResponse], error) {
	s.project = request.Msg.Project
	return connect.NewResponse(&rtestv1.ExchangeGitHubOIDCResponse{Token: "project-session"}), nil
}

func (s *oidcDoctorService) GetServiceInfo(context.Context, *connect.Request[rtestv1.GetServiceInfoRequest]) (*connect.Response[rtestv1.GetServiceInfoResponse], error) {
	return connect.NewResponse(&rtestv1.GetServiceInfoResponse{Version: "test"}), nil
}

func (s *projectListService) ActivateProjectImage(_ context.Context, request *connect.Request[rtestv1.ActivateProjectImageRequest]) (*connect.Response[rtestv1.ActivateProjectImageResponse], error) {
	s.mu.Lock()
	s.activatedProject, s.activatedImage = request.Msg.Project, request.Msg.Image
	s.mu.Unlock()
	return connect.NewResponse(&rtestv1.ActivateProjectImageResponse{Project: &rtestv1.Project{Slug: request.Msg.Project, ActiveImage: request.Msg.Image}}), nil
}

func (s *projectListService) ListProjects(context.Context, *connect.Request[rtestv1.ListProjectsRequest]) (*connect.Response[rtestv1.ListProjectsResponse], error) {
	return connect.NewResponse(&rtestv1.ListProjectsResponse{Projects: s.projects}), nil
}

func (s *projectListService) PrepareJob(context.Context, *connect.Request[rtestv1.PrepareJobRequest]) (*connect.Response[rtestv1.PrepareJobResponse], error) {
	s.mu.Lock()
	s.prepareCount++
	s.mu.Unlock()
	return nil, connect.NewError(connect.CodeInternal, context.Canceled)
}

func testServiceClient(t *testing.T, service rtestv1connect.ControlServiceHandler) (rtestv1connect.ControlServiceClient, func()) {
	t.Helper()
	path, handler := rtestv1connect.NewControlServiceHandler(service)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	return rtestv1connect.NewControlServiceClient(server.Client(), server.URL), server.Close
}

func (s *interruptedLogService) StreamJobLogs(_ context.Context, request *connect.Request[rtestv1.StreamJobLogsRequest], stream *connect.ServerStream[rtestv1.StreamJobLogsResponse]) error {
	s.mu.Lock()
	s.offsets = append(s.offsets, request.Msg.Offset)
	call := len(s.offsets)
	s.mu.Unlock()
	if call == 1 {
		if err := stream.Send(&rtestv1.StreamJobLogsResponse{Data: []byte("abc"), NextOffset: 3}); err != nil {
			return err
		}
		return connect.NewError(connect.CodeUnavailable, context.DeadlineExceeded)
	}
	exitCode := int32(0)
	return stream.Send(&rtestv1.StreamJobLogsResponse{
		Data: []byte("def"), NextOffset: 6,
		TerminalJob: &rtestv1.Job{Id: "job-1", Status: rtestv1.JobStatus_JOB_STATUS_SUCCEEDED, ExitCode: &exitCode, CreatedAt: timestamppb.Now()},
	})
}
