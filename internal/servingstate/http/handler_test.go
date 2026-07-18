package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRequestServingEnvironmentIgnoresQuerySelection(t *testing.T) {
	request := &stdhttp.Request{URL: &url.URL{RawQuery: "environment=staging"}}
	if got := requestServingEnvironment(request, "prod"); got != "prod" {
		t.Fatalf("request serving environment = %q, want prod", got)
	}
}

func TestWriteEnvironmentConflictIncludesBothEnvironments(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeEnvironmentConflict(recorder, "staging", "prod")
	if recorder.Code != stdhttp.StatusConflict || !strings.Contains(recorder.Body.String(), `"requestedEnvironment":"staging"`) || !strings.Contains(recorder.Body.String(), `"instanceEnvironment":"prod"`) {
		t.Fatalf("environment conflict response = %d %s", recorder.Code, recorder.Body.String())
	}
}
