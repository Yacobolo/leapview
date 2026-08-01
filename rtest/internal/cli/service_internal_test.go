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

type projectListService struct {
	rtestv1connect.UnimplementedControlServiceHandler
	mu           sync.Mutex
	projects     []*rtestv1.Project
	prepareCount int
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
