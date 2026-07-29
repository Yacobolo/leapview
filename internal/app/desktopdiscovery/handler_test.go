package desktopdiscovery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestNewHandlerServesVersionedPublicMetadata(t *testing.T) {
	handler, err := NewHandler(Config{
		CanonicalOrigin: "https://analytics.company.com",
		InstanceID:      "instance_0123456789abcdef0123456789abcdef",
		DisplayName:     "Company Analytics",
		ServerVersion:   "v1.4.2",
	})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, WellKnownPath, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache control = %q, want no-store", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("CORS allow origin = %q, want absent", got)
	}

	var document Document
	if err := json.NewDecoder(recorder.Body).Decode(&document); err != nil {
		t.Fatalf("decode document: %v", err)
	}
	want := Document{
		SchemaVersion:      SchemaVersion,
		CanonicalOrigin:    "https://analytics.company.com",
		InstanceID:         "instance_0123456789abcdef0123456789abcdef",
		DisplayName:        "Company Analytics",
		ServerVersion:      "v1.4.2",
		DesktopProtocolMin: DesktopProtocolVersion,
		DesktopProtocolMax: DesktopProtocolVersion,
		AuthenticationModes: []DesktopAuthenticationMode{
			DesktopAuthenticationModeBrowserSession,
			DesktopAuthenticationModeSystemBrowserPkce,
		},
		Capabilities: []DesktopCapability{DesktopCapabilityRemoteWeb},
	}
	if !reflect.DeepEqual(document, want) {
		t.Fatalf("document = %#v, want %#v", document, want)
	}
}

func TestNewHandlerAllowsDevelopmentLoopbackHTTPOnlyWhenExplicit(t *testing.T) {
	if _, err := NewHandler(Config{
		CanonicalOrigin: "http://localhost:8080",
		InstanceID:      "instance_0123456789abcdef0123456789abcdef",
		DisplayName:     "LeapView",
		ServerVersion:   "dev",
	}); err == nil {
		t.Fatal("new handler accepted insecure loopback origin without explicit development allowance")
	}
	if _, err := NewHandler(Config{
		CanonicalOrigin:   "http://localhost:8080",
		InstanceID:        "instance_0123456789abcdef0123456789abcdef",
		DisplayName:       "LeapView",
		ServerVersion:     "dev",
		AllowLoopbackHTTP: true,
	}); err != nil {
		t.Fatalf("new development handler: %v", err)
	}
}

func TestNewHandlerRejectsUnsafeOrAmbiguousConfiguration(t *testing.T) {
	base := Config{
		CanonicalOrigin: "https://analytics.company.com",
		InstanceID:      "instance_0123456789abcdef0123456789abcdef",
		DisplayName:     "Company Analytics",
		ServerVersion:   "v1.4.2",
	}
	tests := map[string]func(*Config){
		"missing origin":       func(config *Config) { config.CanonicalOrigin = "" },
		"origin path":          func(config *Config) { config.CanonicalOrigin += "/path" },
		"origin query":         func(config *Config) { config.CanonicalOrigin += "?tenant=a" },
		"origin fragment":      func(config *Config) { config.CanonicalOrigin += "#fragment" },
		"origin credentials":   func(config *Config) { config.CanonicalOrigin = "https://user@example.com" },
		"insecure remote":      func(config *Config) { config.CanonicalOrigin = "http://example.com"; config.AllowLoopbackHTTP = true },
		"noncanonical slash":   func(config *Config) { config.CanonicalOrigin += "/" },
		"missing instance id":  func(config *Config) { config.InstanceID = "" },
		"invalid instance id":  func(config *Config) { config.InstanceID = "analytics.company.com" },
		"missing display name": func(config *Config) { config.DisplayName = "" },
		"control character":    func(config *Config) { config.DisplayName = "Company\nAnalytics" },
		"oversized name":       func(config *Config) { config.DisplayName = strings.Repeat("a", MaxDisplayNameBytes+1) },
		"missing version":      func(config *Config) { config.ServerVersion = "" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			config := base
			mutate(&config)
			if _, err := NewHandler(config); err == nil {
				t.Fatalf("NewHandler(%#v) succeeded, want error", config)
			}
		})
	}
}

func TestHandlerRejectsUnsupportedMethods(t *testing.T) {
	handler, err := NewHandler(Config{
		CanonicalOrigin: "https://analytics.company.com",
		InstanceID:      "instance_0123456789abcdef0123456789abcdef",
		DisplayName:     "Company Analytics",
		ServerVersion:   "v1.4.2",
	})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, WellKnownPath, nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
	if got := recorder.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("Allow = %q, want GET", got)
	}
}

func TestGeneratedSchemaMatchesPublicDocumentAndFailureKinds(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	schemaBody, err := os.ReadFile(filepath.Join(
		filepath.Dir(sourceFile),
		"..", "..", "..", "schemas", "desktop", "discovery.schema.json",
	))
	if err != nil {
		t.Fatalf("read generated desktop discovery schema: %v", err)
	}
	var schema struct {
		Definitions map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
			Enum       []string                   `json:"enum"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(schemaBody, &schema); err != nil {
		t.Fatalf("decode generated desktop discovery schema: %v", err)
	}

	documentSchema, ok := schema.Definitions["DesktopDiscoveryDocument"]
	if !ok {
		t.Fatal("generated schema is missing DesktopDiscoveryDocument")
	}
	documentBody, err := json.Marshal(Document{
		SchemaVersion:      SchemaVersion,
		CanonicalOrigin:    "https://analytics.company.com",
		InstanceID:         "instance_0123456789abcdef0123456789abcdef",
		DisplayName:        "Company Analytics",
		ServerVersion:      "v1.4.2",
		DesktopProtocolMin: DesktopProtocolVersion,
		DesktopProtocolMax: DesktopProtocolVersion,
		AuthenticationModes: []DesktopAuthenticationMode{
			DesktopAuthenticationModeBrowserSession,
			DesktopAuthenticationModeSystemBrowserPkce,
		},
		Capabilities: []DesktopCapability{DesktopCapabilityRemoteWeb},
	})
	if err != nil {
		t.Fatalf("marshal generated desktop discovery model: %v", err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(documentBody, &document); err != nil {
		t.Fatalf("decode generated desktop discovery model: %v", err)
	}
	if !reflect.DeepEqual(sortedKeys(document), sortedKeys(documentSchema.Properties)) {
		t.Fatalf(
			"generated response properties = %v, generated schema properties = %v",
			sortedKeys(document),
			sortedKeys(documentSchema.Properties),
		)
	}

	failureKinds := schema.Definitions["DesktopDiscoveryFailureKind"].Enum
	for _, want := range []string{
		"tls",
		"proxy",
		"dns",
		"network",
		"malformed_response",
		"protocol_incompatible",
		"canonical_origin_mismatch",
	} {
		if !slices.Contains(failureKinds, want) {
			t.Fatalf("generated failure kinds %v do not contain %q", failureKinds, want)
		}
	}
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
