// Package http serves the public LibreDash website and documentation portal.
package http

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Yacobolo/libredash/pkg/pagestream"
	siteassets "github.com/Yacobolo/libredash/site"
	"github.com/starfederation/datastar-go/datastar"
)

// NewHandler builds the public site HTTP handler without starting a server.
func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", home)
	mux.HandleFunc("GET /charts", charts)
	mux.HandleFunc("GET /docs", docsIndex)
	mux.HandleFunc("GET /docs/openapi.yaml", docsOpenAPISpecification)
	mux.HandleFunc("GET /docs/schemas/{schema}", docsConfigurationSchema)
	mux.HandleFunc("GET /docs/api/{resource}", docsAPIReference)
	mux.HandleFunc("GET /docs/charts/{chart}", docsChart)
	mux.HandleFunc("GET /docs/config/{resource}", docsConfigurationReference)
	mux.HandleFunc("GET /docs/cli/{command}", docsCLIReference)
	mux.HandleFunc("GET /docs/{article}", docsArticle)
	mux.HandleFunc("GET /getting-started", gettingStarted)
	mux.HandleFunc("GET /updates", updates)
	mux.HandleFunc("POST /demo", updateDemo)
	mux.Handle("GET /static/", compressedAssets(http.StripPrefix("/static/", http.FileServer(http.FS(siteassets.Static())))))
	mux.Handle("GET /shared/", compressedAssets(http.StripPrefix("/shared/", http.FileServer(http.FS(siteassets.Shared())))))
	return mux
}

func compressedAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !acceptsGzip(r.Header.Get("Accept-Encoding")) || r.Header.Get("Range") != "" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		compressed := gzip.NewWriter(w)
		defer compressed.Close()
		next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, writer: compressed}, r)
	})
}

func acceptsGzip(header string) bool {
	for _, value := range strings.Split(header, ",") {
		parts := strings.Split(strings.TrimSpace(value), ";")
		if parts[0] != "gzip" {
			continue
		}
		for _, parameter := range parts[1:] {
			if strings.TrimSpace(parameter) == "q=0" {
				return false
			}
		}
		return true
	}
	return false
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer io.Writer
}

func (w *gzipResponseWriter) WriteHeader(statusCode int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *gzipResponseWriter) Write(contents []byte) (int, error) {
	w.Header().Del("Content-Length")
	return w.writer.Write(contents)
}

func home(w http.ResponseWriter, _ *http.Request) {
	if err := sitePage().Render(w); err != nil {
		http.Error(w, "render site page", http.StatusInternalServerError)
	}
}

func charts(w http.ResponseWriter, _ *http.Request) {
	if err := chartsPage().Render(w); err != nil {
		http.Error(w, "render charts page", http.StatusInternalServerError)
	}
}

func gettingStarted(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/docs/getting-started", http.StatusPermanentRedirect)
}

func docsIndex(w http.ResponseWriter, _ *http.Request) {
	if err := docsIndexPage().Render(w); err != nil {
		http.Error(w, "render docs index", http.StatusInternalServerError)
	}
}

func docsArticle(w http.ResponseWriter, r *http.Request) {
	document, ok := siteDocumentBySlug(r.PathValue("article"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := docsArticlePage(document).Render(w); err != nil {
		http.Error(w, "render documentation article", http.StatusInternalServerError)
	}
}

func docsChart(w http.ResponseWriter, r *http.Request) {
	document, ok := siteDocumentBySlug("charts/" + r.PathValue("chart"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := docsArticlePage(document).Render(w); err != nil {
		http.Error(w, "render chart documentation", http.StatusInternalServerError)
	}
}

func docsAPIReference(w http.ResponseWriter, r *http.Request) {
	document, ok := siteDocumentBySlug("api/" + r.PathValue("resource"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := docsArticlePage(document).Render(w); err != nil {
		http.Error(w, "render API reference", http.StatusInternalServerError)
	}
}

func docsOpenAPISpecification(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	_, _ = w.Write(siteOpenAPISpecification())
}

func docsConfigurationSchema(w http.ResponseWriter, r *http.Request) {
	schema, ok := siteConfigurationSchema(r.PathValue("schema"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/schema+json; charset=utf-8")
	_, _ = w.Write(schema)
}

func docsConfigurationReference(w http.ResponseWriter, r *http.Request) {
	document, ok := siteDocumentBySlug("config/" + r.PathValue("resource"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := docsArticlePage(document).Render(w); err != nil {
		http.Error(w, "render configuration reference", http.StatusInternalServerError)
	}
}

func docsCLIReference(w http.ResponseWriter, r *http.Request) {
	document, ok := siteDocumentBySlug("cli/" + r.PathValue("command"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := docsArticlePage(document).Render(w); err != nil {
		http.Error(w, "render CLI reference", http.StatusInternalServerError)
	}
}

func updates(w http.ResponseWriter, r *http.Request) {
	stream := pagestream.NewSignalStream(w, r)
	patch := demoPatch("revenue")
	if r.URL.Query().Get("view") == "charts" {
		patch = chartShowcasePatch()
	}
	if err := stream.Patch(patch); err != nil {
		return
	}
}

func updateDemo(w http.ResponseWriter, r *http.Request) {
	var signals struct {
		Demo struct {
			Metric string `json:"metric"`
		} `json:"demo"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, "read demo signals", http.StatusBadRequest)
		return
	}
	metric := strings.TrimSpace(signals.Demo.Metric)
	if _, ok := demoMetrics[metric]; !ok {
		http.Error(w, fmt.Sprintf("unknown demo metric %q", metric), http.StatusBadRequest)
		return
	}
	_ = pagestream.PatchResponse(w, r, demoPatch(metric))
}
