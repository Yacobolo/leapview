// Package cliapi defines capability-agnostic facilities used by CLI adapters
// to call the application's generated HTTP API.
package cliapi

import (
	"context"
	"net/url"
)

// Credentials are the optional target and token supplied by a command.
// A Client resolves empty values through application-owned configuration.
type Credentials struct {
	Target string
	Token  string
}

// Request describes one generated API operation without coupling a capability
// adapter to application routing or HTTP client construction.
type Request struct {
	Method      string
	OperationID string
	PathParams  map[string]string
	Query       url.Values
	Headers     map[string]string
	Body        any
}

// Client is the narrow application-facing port used by capability CLI
// adapters. Implementations own credential and transport configuration.
type Client interface {
	Resolve(context.Context, Credentials) (Credentials, error)
	DoJSON(context.Context, Credentials, Request, any) error
}
