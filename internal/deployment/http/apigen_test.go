package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	deploymentgen "github.com/Yacobolo/leapview/internal/deployment/api/gen"
)

var _ deploymentgen.GenOperationDispatcher = (*APIGenDispatcher)(nil)
var _ deploymentgen.GenTransportErrorResponder = APIGenTransportErrorResponder{}

func TestAPIGenDispatcherMapsDeploymentCreateIdempotency(t *testing.T) {
	handler := &recordingDeploymentHandler{}
	NewAPIGenDispatcher(handler).CreateDeployment(
		httptest.NewRecorder(),
		httptest.NewRequest(stdhttp.MethodPost, "/api/v1/projects/p1/deployments", nil),
		"p1",
		deploymentgen.GenCreateDeploymentHeaders{IdempotencyKey: "request-1"},
	)
	if got, want := handler.idempotencyKey, "request-1"; got != want {
		t.Fatalf("idempotency key = %q, want %q", got, want)
	}
}

type recordingDeploymentHandler struct{ idempotencyKey string }

func (*recordingDeploymentHandler) ListDeployments(stdhttp.ResponseWriter, *stdhttp.Request, string, *int32, *string) {
}
func (h *recordingDeploymentHandler) CreateDeployment(_ stdhttp.ResponseWriter, _ *stdhttp.Request, _ string, key string) {
	h.idempotencyKey = key
}
func (*recordingDeploymentHandler) GetDeployment(stdhttp.ResponseWriter, *stdhttp.Request, string, string) {
}
func (*recordingDeploymentHandler) CancelDeployment(stdhttp.ResponseWriter, *stdhttp.Request, string, string) {
}
func (*recordingDeploymentHandler) ListDeploymentEvents(stdhttp.ResponseWriter, *stdhttp.Request, string, string, *int32, *string) {
}
func (*recordingDeploymentHandler) RollbackDeployment(stdhttp.ResponseWriter, *stdhttp.Request, string, string, string) {
}
