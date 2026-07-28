package maliciousinstance

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestDefaultManifestCoversApprovedThreatModel(t *testing.T) {
	t.Parallel()

	manifest := DefaultManifest()
	if err := manifest.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if manifest.Version != ManifestVersion {
		t.Fatalf("Version = %q, want %q", manifest.Version, ManifestVersion)
	}

	got := make([]string, 0, len(manifest.Attacks))
	for _, attack := range manifest.Attacks {
		got = append(got, attack.ID)
	}
	for _, want := range []string{
		"native.wails-global",
		"native.wails-http-transport",
		"native.electron-global",
		"native.tauri-global",
		"navigation.cross-origin",
		"navigation.javascript",
		"navigation.data",
		"navigation.blob",
		"navigation.file",
		"popup.cross-origin",
		"frame.cross-origin",
		"scheme.custom",
		"scheme.deep-link-injection",
		"permission.camera",
		"permission.microphone",
		"permission.geolocation",
		"permission.notifications",
		"permission.clipboard-read",
		"download.hostile-filename",
		"storage.cross-profile",
		"discovery.malformed",
		"discovery.oversized",
		"renderer.resource-exhaustion",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("manifest attack IDs = %v, missing %q", got, want)
		}
	}
}

func TestManifestValidationRejectsAmbiguousAttackContracts(t *testing.T) {
	t.Parallel()

	valid := DefaultManifest()
	tests := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{name: "missing version", mutate: func(manifest *Manifest) { manifest.Version = "" }},
		{name: "unknown version", mutate: func(manifest *Manifest) { manifest.Version = "future" }},
		{name: "no attacks", mutate: func(manifest *Manifest) { manifest.Attacks = nil }},
		{name: "duplicate attack", mutate: func(manifest *Manifest) {
			manifest.Attacks = append(manifest.Attacks, manifest.Attacks[0])
		}},
		{name: "missing path", mutate: func(manifest *Manifest) { manifest.Attacks[0].Path = "" }},
		{name: "invalid trigger", mutate: func(manifest *Manifest) { manifest.Attacks[0].Trigger = Trigger("sometimes") }},
		{name: "invalid expected outcome", mutate: func(manifest *Manifest) {
			manifest.Attacks[0].Expected = Outcome("maybe")
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := valid
			manifest.Attacks = append([]Attack(nil), valid.Attacks...)
			test.mutate(&manifest)
			if err := manifest.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
		})
	}
}

func TestNewRejectsAmbiguousHarnessConfiguration(t *testing.T) {
	t.Parallel()

	for _, config := range []Config{
		{},
		{ExternalOrigin: "file:///tmp/attacker"},
		{ExternalOrigin: "https://user:secret@attacker.example"},
		{ExternalOrigin: "https://attacker.example/path"},
		{ExternalOrigin: "https://attacker.example?next=target"},
		{ExternalOrigin: "https://attacker.example", OversizedResponseBytes: -1},
		{ExternalOrigin: "https://attacker.example", OversizedResponseBytes: maxOversizedResponseBytes + 1},
	} {
		if _, err := New(config); err == nil {
			t.Errorf("New(%#v) error = nil, want error", config)
		}
	}
}

func TestHandlerPublishesDeterministicManifestAndProbePage(t *testing.T) {
	t.Parallel()

	harness := newTestHarness(t)
	server := httptest.NewServer(harness.Handler())
	t.Cleanup(server.Close)

	response := get(t, server.URL+"/__harness/manifest.json")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("manifest status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if got := response.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var manifest Manifest
	if err := json.NewDecoder(response.Body).Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	if err := manifest.Validate(); err != nil {
		t.Fatalf("published manifest is invalid: %v", err)
	}

	page := get(t, server.URL+"/")
	defer page.Body.Close()
	body, err := io.ReadAll(page.Body)
	if err != nil {
		t.Fatal(err)
	}
	if page.StatusCode != http.StatusOK {
		t.Fatalf("page status = %d, want %d", page.StatusCode, http.StatusOK)
	}
	if got := page.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if got := page.Header.Get("Content-Security-Policy"); !strings.Contains(got, "connect-src *") {
		t.Fatalf("Content-Security-Policy = %q, want intentionally permissive connect-src", got)
	}
	for _, want := range []string{`data-harness-version="` + ManifestVersion + `"`, "/__harness/probe.js"} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("probe page missing %q", want)
		}
	}
}

func TestProbeScriptNamesKnownNativeSurfaces(t *testing.T) {
	t.Parallel()

	harness := newTestHarness(t)
	recorder := httptest.NewRecorder()
	harness.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/__harness/probe.js", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	for _, want := range []string{
		"window._wails",
		"window.wails",
		"window.chrome.webview",
		"window.webkit.messageHandlers.external",
		"window.electron",
		"window.__TAURI__",
		"http://wails.localhost/wails/runtime",
	} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Errorf("probe script missing %q", want)
		}
	}
}

func TestObservationCollectorAcceptsOnlyBoundedStructuredEvidence(t *testing.T) {
	t.Parallel()

	harness := newTestHarness(t)
	server := httptest.NewServer(harness.Handler())
	t.Cleanup(server.Close)

	postReport(t, server.URL, http.StatusNoContent, `{
		"runId":"run-1",
		"observations":[
			{"attackId":"native.wails-global","outcome":"blocked"},
			{"attackId":"native.wails-http-transport","outcome":"exposed"}
		]
	}`)

	reports := harness.Reports()
	if len(reports) != 1 {
		t.Fatalf("Reports() = %#v, want one report", reports)
	}
	if reports[0].RunID != "run-1" || len(reports[0].Observations) != 2 {
		t.Fatalf("report = %#v", reports[0])
	}

	reports[0].RunID = "mutated"
	if got := harness.Reports()[0].RunID; got != "run-1" {
		t.Fatalf("Reports() returned mutable state: RunID = %q", got)
	}

	harness.Reset()
	if got := harness.Reports(); len(got) != 0 {
		t.Fatalf("Reports() after Reset() = %#v, want empty", got)
	}
}

func TestHarnessReturnsCopySafeManifest(t *testing.T) {
	t.Parallel()

	harness := newTestHarness(t)
	manifest := harness.Manifest()
	manifest.Version = "mutated"
	manifest.Attacks[0].ID = "mutated"

	fresh := harness.Manifest()
	if fresh.Version != ManifestVersion || fresh.Attacks[0].ID == "mutated" {
		t.Fatalf("Manifest() returned mutable state: %#v", fresh)
	}
}

func TestObservationCollectorRejectsUntrustedOrAmbiguousEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "unknown field", body: `{"runId":"run-1","secret":"token","observations":[]}`},
		{name: "invalid run ID", body: `{"runId":"../../../secret","observations":[]}`},
		{name: "unknown attack", body: `{"runId":"run-1","observations":[{"attackId":"native.unknown","outcome":"blocked"}]}`},
		{name: "invalid outcome", body: `{"runId":"run-1","observations":[{"attackId":"native.wails-global","outcome":"maybe"}]}`},
		{name: "duplicate attack", body: `{"runId":"run-1","observations":[{"attackId":"native.wails-global","outcome":"blocked"},{"attackId":"native.wails-global","outcome":"exposed"}]}`},
		{name: "free-form detail", body: `{"runId":"run-1","observations":[{"attackId":"native.wails-global","outcome":"blocked","detail":"session=secret"}]}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness := newTestHarness(t)
			server := httptest.NewServer(harness.Handler())
			t.Cleanup(server.Close)
			postReport(t, server.URL, http.StatusBadRequest, test.body)
			if got := harness.Reports(); len(got) != 0 {
				t.Fatalf("Reports() = %#v, want empty", got)
			}
		})
	}

	harness := newTestHarness(t)
	oversizedBody := `{"runId":"run-1","observations":[]}` + strings.Repeat(" ", maxReportBytes)
	request := httptest.NewRequest(http.MethodPost, "/__harness/report", strings.NewReader(oversizedBody))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	harness.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized report status = %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestObservationCollectorBoundsStoredReports(t *testing.T) {
	t.Parallel()

	harness := newTestHarness(t)
	server := httptest.NewServer(harness.Handler())
	t.Cleanup(server.Close)

	for index := range maxStoredReports {
		postReport(t, server.URL, http.StatusNoContent, `{"runId":"run-`+string(rune('A'+index%26))+`","observations":[]}`)
	}
	postReport(t, server.URL, http.StatusTooManyRequests, `{"runId":"run-overflow","observations":[]}`)
	if got := len(harness.Reports()); got != maxStoredReports {
		t.Fatalf("stored reports = %d, want %d", got, maxStoredReports)
	}
}

func TestAttackRoutesExposeControlledHostileBehaviors(t *testing.T) {
	t.Parallel()

	const externalOrigin = "https://attacker.example"
	harness, err := New(Config{ExternalOrigin: externalOrigin, OversizedResponseBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	handler := harness.Handler()

	redirect := httptest.NewRecorder()
	handler.ServeHTTP(redirect, httptest.NewRequest(http.MethodGet, "/attack/navigation.cross-origin", nil))
	if redirect.Code != http.StatusTemporaryRedirect {
		t.Fatalf("redirect status = %d, want %d", redirect.Code, http.StatusTemporaryRedirect)
	}
	if got := redirect.Header().Get("Location"); got != externalOrigin+"/desktop-harness-target" {
		t.Fatalf("redirect Location = %q", got)
	}

	malformed := httptest.NewRecorder()
	handler.ServeHTTP(malformed, httptest.NewRequest(http.MethodGet, "/attack/discovery.malformed", nil))
	if malformed.Code != http.StatusOK || json.Valid(malformed.Body.Bytes()) {
		t.Fatalf("malformed discovery = status %d body %q, want invalid JSON", malformed.Code, malformed.Body.String())
	}

	oversized := httptest.NewRecorder()
	handler.ServeHTTP(oversized, httptest.NewRequest(http.MethodGet, "/attack/discovery.oversized", nil))
	if oversized.Code != http.StatusOK || oversized.Body.Len() != 4096 {
		t.Fatalf("oversized discovery = status %d bytes %d, want 4096 bytes", oversized.Code, oversized.Body.Len())
	}

	download := httptest.NewRecorder()
	handler.ServeHTTP(download, httptest.NewRequest(http.MethodGet, "/attack/download.hostile-filename", nil))
	if got := download.Header().Get("Content-Disposition"); !strings.Contains(got, "..") {
		t.Fatalf("Content-Disposition = %q, want hostile filename fixture", got)
	}

	permission := httptest.NewRecorder()
	handler.ServeHTTP(permission, httptest.NewRequest(http.MethodGet, "/attack/permission.camera", nil))
	if strings.Contains(permission.Body.String(), "cross-origin-frame") {
		t.Fatal("permission attack page unexpectedly includes the frame attack")
	}

	frame := httptest.NewRecorder()
	handler.ServeHTTP(frame, httptest.NewRequest(http.MethodGet, "/attack/frame.cross-origin", nil))
	if !strings.Contains(frame.Body.String(), "cross-origin-frame") {
		t.Fatal("frame attack page is missing the cross-origin frame")
	}

	native := httptest.NewRecorder()
	handler.ServeHTTP(native, httptest.NewRequest(http.MethodGet, "/attack/native.wails-global", nil))
	if !strings.Contains(native.Body.String(), "/__harness/probe.js") {
		t.Fatal("native attack page is missing the automatic native-surface probe")
	}

	for _, attack := range []struct {
		id      string
		payload string
	}{
		{id: "navigation.blob", payload: "URL.createObjectURL"},
		{id: "scheme.deep-link-injection", payload: "leapview-desktop://open"},
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/attack/"+attack.id, nil))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), attack.payload) {
			t.Errorf("%s page = status %d body %q, want payload %q", attack.id, recorder.Code, recorder.Body.String(), attack.payload)
		}
	}

	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/unknown", nil))
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown route status = %d, want %d", unknown.Code, http.StatusNotFound)
	}
}

func newTestHarness(t *testing.T) *Harness {
	t.Helper()
	harness, err := New(Config{ExternalOrigin: "https://attacker.example"})
	if err != nil {
		t.Fatal(err)
	}
	return harness
}

func newHarnessServer(t *testing.T, harness *Harness) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(harness.Handler())
	t.Cleanup(server.Close)
	return server
}

func newExternalTargetServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!doctype html><title>External hostile target</title>")
	}))
	t.Cleanup(server.Close)
	return server
}

func get(t *testing.T, url string) *http.Response {
	t.Helper()
	response, err := http.Get(url) //nolint:gosec // Test-only loopback server.
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func postReport(t *testing.T, serverURL string, wantStatus int, body string) {
	t.Helper()
	response, err := http.Post(serverURL+"/__harness/report", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != wantStatus {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("status = %d, want %d: %s", response.StatusCode, wantStatus, payload)
	}
}
