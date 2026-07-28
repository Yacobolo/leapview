// Package desktopdiscovery serves the bounded, unauthenticated compatibility
// document used before a LeapView desktop client trusts an instance.
package desktopdiscovery

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	WellKnownPath          = "/.well-known/leapview"
	SchemaVersion          = 1
	DesktopProtocolVersion = 1
	MaxDisplayNameBytes    = 120
	maxServerVersionBytes  = 64
)

var instanceIDPattern = regexp.MustCompile(`^instance_[0-9a-f]{32}$`)

type Config struct {
	CanonicalOrigin   string
	InstanceID        string
	DisplayName       string
	ServerVersion     string
	AllowLoopbackHTTP bool
}

type Document struct {
	SchemaVersion       int      `json:"schemaVersion"`
	CanonicalOrigin     string   `json:"canonicalOrigin"`
	InstanceID          string   `json:"instanceId"`
	DisplayName         string   `json:"displayName"`
	ServerVersion       string   `json:"serverVersion"`
	DesktopProtocolMin  int      `json:"desktopProtocolMin"`
	DesktopProtocolMax  int      `json:"desktopProtocolMax"`
	AuthenticationModes []string `json:"authenticationModes"`
	Capabilities        []string `json:"capabilities"`
}

func NewHandler(config Config) (http.Handler, error) {
	origin, err := validateCanonicalOrigin(config.CanonicalOrigin, config.AllowLoopbackHTTP)
	if err != nil {
		return nil, err
	}
	if !instanceIDPattern.MatchString(config.InstanceID) {
		return nil, fmt.Errorf("instance id must be an opaque LeapView instance id")
	}
	displayName, err := validateDisplayString("display name", config.DisplayName, MaxDisplayNameBytes)
	if err != nil {
		return nil, err
	}
	serverVersion, err := validateDisplayString("server version", config.ServerVersion, maxServerVersionBytes)
	if err != nil {
		return nil, err
	}
	document := Document{
		SchemaVersion:       SchemaVersion,
		CanonicalOrigin:     origin,
		InstanceID:          config.InstanceID,
		DisplayName:         displayName,
		ServerVersion:       serverVersion,
		DesktopProtocolMin:  DesktopProtocolVersion,
		DesktopProtocolMax:  DesktopProtocolVersion,
		AuthenticationModes: []string{"browser-session", "system-browser-pkce"},
		Capabilities:        []string{"remote-web"},
	}
	body, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode desktop discovery document: %w", err)
	}
	body = append(body, '\n')
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}), nil
}

func validateCanonicalOrigin(raw string, allowLoopbackHTTP bool) (string, error) {
	if raw == "" || strings.TrimSpace(raw) != raw {
		return "", fmt.Errorf("canonical origin is required without surrounding whitespace")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse canonical origin: %w", err)
	}
	if parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" ||
		parsed.Path != "" || parsed.RawPath != "" || parsed.RawQuery != "" ||
		parsed.ForceQuery || parsed.Fragment != "" {
		return "", fmt.Errorf("canonical origin must contain only a scheme and authority")
	}
	switch parsed.Scheme {
	case "https":
	case "http":
		if !allowLoopbackHTTP || !isLoopbackHost(parsed.Hostname()) {
			return "", fmt.Errorf("canonical origin must use HTTPS")
		}
	default:
		return "", fmt.Errorf("canonical origin must use HTTPS")
	}
	if parsed.String() != raw {
		return "", fmt.Errorf("canonical origin is not in canonical form")
	}
	return raw, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validateDisplayString(name, value string, maxBytes int) (string, error) {
	if value == "" || strings.TrimSpace(value) != value {
		return "", fmt.Errorf("%s is required without surrounding whitespace", name)
	}
	if !utf8.ValidString(value) || len(value) > maxBytes {
		return "", fmt.Errorf("%s must be valid UTF-8 and at most %d bytes", name, maxBytes)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return "", fmt.Errorf("%s must not contain control characters", name)
		}
	}
	return value, nil
}
