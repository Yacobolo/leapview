package mcpoauth

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxClientMetadataBytes = 1 << 20

type clientMetadataDocument struct {
	ClientID                string   `json:"client_id"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

func (s *Service) resolveClientMetadata(ctx context.Context, clientID string) (storedClient, error) {
	parsed, err := url.Parse(clientID)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Path == "" || parsed.Path == "/" || parsed.User != nil || parsed.Fragment != "" {
		return storedClient{}, fmt.Errorf("CIMD client_id must be an HTTPS metadata URL with a path")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return storedClient{}, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := s.metadataClient.Do(request)
	if err != nil {
		return storedClient{}, fmt.Errorf("fetch client metadata: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return storedClient{}, fmt.Errorf("client metadata returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxClientMetadataBytes+1))
	if err != nil || len(body) > maxClientMetadataBytes {
		return storedClient{}, fmt.Errorf("client metadata exceeds the size limit")
	}
	var metadata clientMetadataDocument
	if err := json.Unmarshal(body, &metadata); err != nil {
		return storedClient{}, fmt.Errorf("decode client metadata: %w", err)
	}
	if metadata.ClientID != clientID {
		return storedClient{}, fmt.Errorf("client metadata client_id does not match its URL")
	}
	metadata.ClientName = strings.TrimSpace(metadata.ClientName)
	if metadata.ClientName == "" || len(metadata.ClientName) > 200 || len(metadata.RedirectURIs) == 0 || len(metadata.RedirectURIs) > 10 {
		return storedClient{}, fmt.Errorf("client metadata requires a bounded client_name and redirect_uris")
	}
	for _, redirect := range metadata.RedirectURIs {
		if err := validateCanonicalURL(redirect, true); err != nil {
			return storedClient{}, fmt.Errorf("invalid client metadata redirect URI: %w", err)
		}
	}
	if len(metadata.GrantTypes) == 0 {
		metadata.GrantTypes = []string{"authorization_code"}
	}
	if len(metadata.ResponseTypes) == 0 {
		metadata.ResponseTypes = []string{"code"}
	}
	if metadata.TokenEndpointAuthMethod == "" {
		metadata.TokenEndpointAuthMethod = "none"
	}
	if !allowedValues(metadata.GrantTypes, "authorization_code", "refresh_token") ||
		!allowedValues(metadata.ResponseTypes, "code") || metadata.TokenEndpointAuthMethod != "none" {
		return storedClient{}, fmt.Errorf("CIMD client must be a public authorization-code client")
	}
	return storedClient{
		ID: clientID, Name: metadata.ClientName, RedirectURIs: metadata.RedirectURIs,
		GrantTypes: metadata.GrantTypes, ResponseTypes: metadata.ResponseTypes,
		Scopes: []string{ScopeMCPUse, ScopeOfflineAccess}, Audience: []string{s.config.ResourceURL},
		Public: true, TokenEndpointAuthMethod: "none",
	}, nil
}

func secureClientMetadataHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 nil,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		ResponseHeaderTimeout: 5 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, address := range addresses {
			if !publicIP(address.IP) {
				continue
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
		}
		return nil, fmt.Errorf("client metadata host has no public IP address")
	}
	return &http.Client{
		Transport: transport,
		Timeout:   7 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func publicIP(ip net.IP) bool {
	return ip != nil && !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsUnspecified() &&
		!ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && !ip.IsMulticast()
}
