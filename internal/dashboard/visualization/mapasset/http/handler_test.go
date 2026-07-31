package mapassethttp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	visualizationmapasset "github.com/flidai/leapview/internal/dashboard/visualization/mapasset"
)

func TestCacheHandlerServesRealImmutableByteRanges(t *testing.T) {
	root := t.TempDir()
	asset, err := visualizationmapasset.Resolve("streets")
	if err != nil {
		t.Fatal(err)
	}
	relative := asset.ArchiveURL[len("/map-assets/"):]
	name := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := CacheHandler(http.StripPrefix("/map-assets/", http.FileServer(http.Dir(root))))
	request := httptest.NewRequest(http.MethodGet, asset.ArchiveURL, nil)
	request.Header.Set("Range", "bytes=2-5")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusPartialContent)
	}
	body, err := io.ReadAll(response.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "2345" || response.Header().Get("Content-Range") != "bytes 2-5/10" {
		t.Fatalf("range response = %q, Content-Range %q", body, response.Header().Get("Content-Range"))
	}
	if response.Header().Get("Cache-Control") != visualizationmapasset.ImmutableCacheControl || response.Header().Get("Accept-Ranges") != "bytes" || response.Header().Get("Content-Type") != "application/vnd.pmtiles" {
		t.Fatalf("asset headers = %#v", response.Header())
	}
	if got, want := response.Header().Get("ETag"), `"sha256-`+visualizationmapasset.ArchiveSHA256+`"`; got != want {
		t.Fatalf("ETag = %q, want %q", got, want)
	}
}

func TestCacheHandlerServesEmbeddedWorldwideArchiveRanges(t *testing.T) {
	asset, err := visualizationmapasset.Resolve("streets")
	if err != nil {
		t.Fatal(err)
	}
	handler := CacheHandler(http.StripPrefix("/map-assets/", http.FileServer(http.FS(visualizationmapasset.EmbeddedFS()))))
	request := httptest.NewRequest(http.MethodGet, asset.ArchiveURL, nil)
	request.Header.Set("Range", "bytes=0-126")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusPartialContent)
	}
	body, err := io.ReadAll(response.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 127 || string(body[:7]) != "PMTiles" {
		t.Fatalf("embedded PMTiles header = %q (%d bytes)", body[:min(len(body), 7)], len(body))
	}
	if got, want := response.Header().Get("Content-Range"), "bytes 0-126/44725293"; got != want {
		t.Fatalf("Content-Range = %q, want %q", got, want)
	}
	if got, want := response.Header().Get("ETag"), `"sha256-`+visualizationmapasset.ArchiveSHA256+`"`; got != want {
		t.Fatalf("ETag = %q, want %q", got, want)
	}
}

func TestCacheHandlerServesEmbeddedStyleAsACompleteImmutableResponse(t *testing.T) {
	asset, err := visualizationmapasset.Resolve("streets")
	if err != nil {
		t.Fatal(err)
	}
	handler := CacheHandler(http.StripPrefix("/map-assets/", http.FileServer(http.FS(visualizationmapasset.EmbeddedFS()))))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, asset.StyleURL, nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Body.Len() == 0 || response.Header().Get("Accept-Ranges") != "bytes" || response.Header().Get("Cache-Control") != visualizationmapasset.ImmutableCacheControl {
		t.Fatalf("complete style response headers = %#v bytes = %d", response.Header(), response.Body.Len())
	}
	if got, want := response.Header().Get("ETag"), `"sha256-`+visualizationmapasset.StyleSHA256+`"`; got != want {
		t.Fatalf("ETag = %q, want %q", got, want)
	}
}

func TestCacheHandlerRejectsMutablePathsAndMutationMethods(t *testing.T) {
	called := false
	handler := CacheHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))

	mutable := httptest.NewRecorder()
	handler.ServeHTTP(mutable, httptest.NewRequest(http.MethodGet, "/map-assets/leapview-streets/basemap.pmtiles", nil))
	if mutable.Code != http.StatusNotFound || mutable.Header().Get("Cache-Control") != "no-store" || called {
		t.Fatalf("mutable request = status %d headers %#v called %v", mutable.Code, mutable.Header(), called)
	}

	asset, err := visualizationmapasset.Resolve("streets")
	if err != nil {
		t.Fatal(err)
	}
	mutation := httptest.NewRecorder()
	handler.ServeHTTP(mutation, httptest.NewRequest(http.MethodPost, asset.ArchiveURL, nil))
	if mutation.Code != http.StatusMethodNotAllowed || mutation.Header().Get("Allow") != "GET, HEAD" || called {
		t.Fatalf("mutation request = status %d headers %#v called %v", mutation.Code, mutation.Header(), called)
	}
}
