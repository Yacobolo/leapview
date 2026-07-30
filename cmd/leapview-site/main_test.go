package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseBaseURL(t *testing.T) {
	for _, test := range []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "empty for development", raw: ""},
		{name: "HTTPS origin", raw: "https://docs.leapview.dev/", want: "https://docs.leapview.dev"},
		{name: "HTTP origin", raw: "http://localhost:8081", want: "http://localhost:8081"},
		{name: "relative", raw: "/docs", wantErr: true},
		{name: "unsupported scheme", raw: "ftp://docs.leapview.dev", wantErr: true},
		{name: "path", raw: "https://docs.leapview.dev/docs", wantErr: true},
		{name: "query", raw: "https://docs.leapview.dev?preview=1", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			baseURL, err := parseBaseURL(test.raw)
			if test.wantErr {
				if err == nil {
					t.Fatalf("parseBaseURL(%q) succeeded, want error", test.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseBaseURL(%q): %v", test.raw, err)
			}
			if baseURL == nil {
				if test.want != "" {
					t.Fatalf("parseBaseURL(%q) = nil, want %q", test.raw, test.want)
				}
				return
			}
			if got := baseURL.String(); got != test.want {
				t.Fatalf("parseBaseURL(%q) = %q, want %q", test.raw, got, test.want)
			}
		})
	}
}

func TestParseShowcaseEmbedURL(t *testing.T) {
	for _, test := range []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{raw: "", want: ""},
		{raw: "https://app.leapview.dev/embed/dashboards/public-id", want: "https://app.leapview.dev/embed/dashboards/public-id"},
		{raw: "http://localhost:8080/embed/dashboards/public-id", want: "http://localhost:8080/embed/dashboards/public-id"},
		{raw: "ftp://app.leapview.dev/embed/dashboards/id", wantErr: true},
		{raw: "https://user:secret@app.leapview.dev/embed/dashboards/id", wantErr: true},
		{raw: "https://app.leapview.dev/embed/dashboards/id?theme=x", wantErr: true},
		{raw: "https://app.leapview.dev/embed/dashboards/id#x", wantErr: true},
		{raw: "https://app.leapview.dev/not-an-embed", wantErr: true},
	} {
		got, err := parseShowcaseEmbedURL(test.raw)
		if test.wantErr {
			if err == nil {
				t.Errorf("parseShowcaseEmbedURL(%q) succeeded", test.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseShowcaseEmbedURL(%q): %v", test.raw, err)
			continue
		}
		if got == nil {
			if test.want != "" {
				t.Errorf("parseShowcaseEmbedURL(%q) = nil", test.raw)
			}
			continue
		}
		if got.String() != test.want {
			t.Errorf("parseShowcaseEmbedURL(%q) = %q, want %q", test.raw, got, test.want)
		}
	}
}

func TestMapAssetsRootFromEnvironmentAllowsEmbeddedDevelopmentFallback(t *testing.T) {
	t.Setenv("LEAPVIEW_SITE_MAP_ASSETS_ROOT", "")
	if got := mapAssetsRootFromEnvironment(); got != "" {
		t.Fatalf("mapAssetsRootFromEnvironment() = %q, want empty development fallback", got)
	}
	root := filepath.Join(t.TempDir(), "map-assets")
	t.Setenv("LEAPVIEW_SITE_MAP_ASSETS_ROOT", "  "+root+"  ")
	if got := mapAssetsRootFromEnvironment(); got != root {
		t.Fatalf("mapAssetsRootFromEnvironment() = %q, want %q", got, root)
	}
}

func TestRunFailsBeforeListeningWhenMapAssetPackageIsInvalid(t *testing.T) {
	if err := run(context.Background(), "127.0.0.1:0", nil, nil, "relative"); err == nil || !strings.Contains(err.Error(), "must be absolute") {
		t.Fatalf("run(relative root) error = %v, want absolute-path diagnostic", err)
	}
	root := filepath.Join(t.TempDir(), "missing")
	if err := run(context.Background(), "127.0.0.1:0", nil, nil, root); err == nil || !strings.Contains(err.Error(), "map asset") {
		t.Fatalf("run(missing root) error = %v, want map-asset diagnostic", err)
	}
}
