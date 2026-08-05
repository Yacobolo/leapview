package connectors

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// QuackURI returns the canonical remote URI accepted by DuckDB's Quack
// extension. The endpoint is deliberately assembled from target-owned host and
// port fields so project configuration cannot smuggle credentials or paths into
// the URI.
func QuackURI(host string, port int) (string, error) {
	if host == "" || host != strings.TrimSpace(host) || port <= 0 || port > 65535 {
		return "", fmt.Errorf("Quack requires a canonical host and port")
	}
	if strings.ContainsAny(host, "/?#@") || strings.Contains(host, "://") {
		return "", fmt.Errorf("Quack host must not contain a scheme, path, query, or user information")
	}
	if strings.Contains(host, ":") && net.ParseIP(host) == nil {
		return "", fmt.Errorf("Quack host contains an invalid colon")
	}
	return "quack:" + net.JoinHostPort(host, strconv.Itoa(port)), nil
}
