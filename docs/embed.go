// Package docs owns the Markdown source for the public documentation site.
package docs

import (
	"embed"
	_ "embed"
)

// GettingStarted is the source for the site's getting-started article.
//
//go:embed getting-started.md
var GettingStarted string

// Configuration is the source for the site's configuration reference.
//
//go:embed configuration.md
var Configuration string

// EnterpriseAuth is the source for the site's enterprise authentication guide.
//
//go:embed enterprise-auth.md
var EnterpriseAuth string

// StorageArchitecture is the source for the site's storage architecture article.
//
//go:embed storage-architecture-spec.md
var StorageArchitecture string

// Visuals contains the Markdown articles for each supported visual type.
//
//go:embed visuals/*.md visuals/*.json
var Visuals embed.FS

// API contains Markdown generated from api/gen/openapi.yaml.
//
//go:embed api/*.md api/*.json api/*.yaml
var API embed.FS
