package module

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	visualizationmapasset "github.com/flidai/leapview/internal/dashboard/visualization/mapasset"
)

func TestBuildAssetsAlwaysServesTheEmbeddedWorldwidePackage(t *testing.T) {
	assets, err := BuildAssets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if assets == nil {
		t.Fatal("BuildAssets() returned nil")
	}
	if err := assets.Verify(context.Background()); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	manifest, err := visualizationmapasset.Resolve("streets")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, manifest.ArchiveURL, nil)
	request.Header.Set("Range", "bytes=0-6")
	response := httptest.NewRecorder()
	assets.Handler().ServeHTTP(response, request)
	body, err := io.ReadAll(response.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusPartialContent || string(body) != "PMTiles" {
		t.Fatalf("embedded archive response = status %d body %q", response.Code, body)
	}
}
