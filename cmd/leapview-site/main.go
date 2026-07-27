package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	sitehttp "github.com/Yacobolo/leapview/internal/app/site/http"
)

func main() {
	address := flag.String("addr", ":8081", "listen address")
	baseURLFlag := flag.String("base-url", os.Getenv("LEAPVIEW_SITE_BASE_URL"), "externally visible site origin (or LEAPVIEW_SITE_BASE_URL)")
	showcaseEmbedURLFlag := flag.String("showcase-embed-url", os.Getenv("LEAPVIEW_SITE_SHOWCASE_EMBED_URL"), "live showcase dashboard embed URL (or LEAPVIEW_SITE_SHOWCASE_EMBED_URL)")
	flag.Parse()

	baseURL, err := parseBaseURL(*baseURLFlag)
	if err != nil {
		log.Fatal(err)
	}
	showcaseEmbedURL, err := parseShowcaseEmbedURL(*showcaseEmbedURLFlag)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *address, baseURL, showcaseEmbedURL); err != nil {
		log.Fatal(err)
	}
}

func parseShowcaseEmbedURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("parse showcase embed URL: %w", err)
	}
	publicID := strings.TrimPrefix(parsed.Path, "/embed/dashboards/")
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || publicID == parsed.Path || publicID == "" || strings.Contains(publicID, "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("showcase embed URL must be an HTTP(S) URL without credentials, query, or fragment: %q", raw)
	}
	return parsed, nil
}

func parseBaseURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("parse site base URL: %w", err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("site base URL must be an HTTP(S) origin without a path, query, credentials, or fragment: %q", raw)
	}
	parsed.Path = ""
	return parsed, nil
}

func run(ctx context.Context, address string, baseURL, showcaseEmbedURL *url.URL) error {
	server := &http.Server{
		Addr:              address,
		Handler:           sitehttp.NewHandlerWithOptions(sitehttp.Options{BaseURL: baseURL, ShowcaseEmbedURL: showcaseEmbedURL}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("LeapView site listening on %s", address)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve public site: %w", err)
	case <-ctx.Done():
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shut down public site: %w", err)
	}
	return nil
}
