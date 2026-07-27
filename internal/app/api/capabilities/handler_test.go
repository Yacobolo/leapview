package capabilities

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yacobolo/leapview/internal/platform/buildinfo"
)

func TestWriteReportsRuntimeBuildIdentity(t *testing.T) {
	recorder := httptest.NewRecorder()
	identity := buildinfo.Identity{
		Version: "0.2.0-rc.1", Revision: strings.Repeat("d", 40),
		BuildTime: "2026-07-27T09:00:00Z",
	}
	Write(recorder, Config{Environment: "prod", BuildIdentity: identity})

	var response struct {
		BuildVersion     string `json:"buildVersion"`
		BuildRevision    string `json:"buildRevision"`
		BuildTime        string `json:"buildTime"`
		BuildDirty       bool   `json:"buildDirty"`
		BuildDevelopment bool   `json:"buildDevelopment"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.BuildVersion != identity.Version ||
		response.BuildRevision != identity.Revision ||
		response.BuildTime != identity.BuildTime ||
		response.BuildDirty != identity.Dirty ||
		response.BuildDevelopment != identity.Development {
		t.Fatalf("capabilities build identity = %#v", response)
	}
}
