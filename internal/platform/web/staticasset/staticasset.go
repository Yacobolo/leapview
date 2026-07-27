package staticasset

import (
	"net/url"
	"os"
	"strings"
)

const DatastarScriptPath = "/static/vendor/datastar-1.0.2.js"

type Config struct {
	Production           bool
	Version              string
	GeneratedVersionPath string
}

// Resolver is an immutable snapshot of static-asset runtime settings.
type Resolver struct {
	production bool
	version    string
}

func New(config Config) Resolver {
	version := strings.TrimSpace(config.Version)
	if version == "" && config.Production {
		if path := strings.TrimSpace(config.GeneratedVersionPath); path != "" {
			if bytes, err := os.ReadFile(path); err == nil {
				version = strings.TrimSpace(string(bytes))
			}
		}
	}
	if version == "" {
		version = "dev"
	}
	return Resolver{production: config.Production, version: version}
}

func (r Resolver) URL(path string) string {
	return path + "?v=" + url.QueryEscape(r.Version())
}

func (r Resolver) Version() string {
	if r.version == "" {
		return "dev"
	}
	return r.version
}

func (r Resolver) Production() bool {
	return r.production
}
