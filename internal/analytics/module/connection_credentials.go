package module

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/flidai/leapview/internal/analytics/connectionbinding"
	"github.com/flidai/leapview/internal/analytics/infisical"
)

type TargetCredentialConfig struct {
	InfisicalBaseURL               string
	InfisicalUniversalClientID     string
	InfisicalUniversalClientSecret string `json:"-" yaml:"-"`
	InfisicalAllowedScopes         string
}

func (TargetCredentialConfig) String() string   { return "<target-credential-config:redacted>" }
func (TargetCredentialConfig) GoString() string { return "module.TargetCredentialConfig{<redacted>}" }

func (config TargetCredentialConfig) configured() bool {
	return strings.TrimSpace(config.InfisicalBaseURL) != "" ||
		strings.TrimSpace(config.InfisicalUniversalClientID) != "" ||
		config.InfisicalUniversalClientSecret != "" ||
		strings.TrimSpace(config.InfisicalAllowedScopes) != ""
}

func buildTargetResolvers(config TargetCredentialConfig) (connectionbinding.ResolverSet, error) {
	if !config.configured() {
		return connectionbinding.ResolverSet{}, nil
	}
	client := &http.Client{Timeout: 15 * time.Second}
	var scopes []struct {
		ProjectID        string `json:"projectId"`
		Environment      string `json:"environment"`
		SecretPathPrefix string `json:"secretPathPrefix"`
	}
	if err := json.Unmarshal([]byte(config.InfisicalAllowedScopes), &scopes); err != nil || len(scopes) == 0 {
		return connectionbinding.ResolverSet{}, fmt.Errorf("%w: Infisical allowed scopes must be a non-empty JSON array", connectionbinding.ErrInvalidBinding)
	}
	allowed := make([]infisical.AllowedScope, len(scopes))
	for index, scope := range scopes {
		allowed[index] = infisical.AllowedScope{
			ProjectID: scope.ProjectID, Environment: scope.Environment, SecretPathPrefix: scope.SecretPathPrefix,
		}
	}
	authenticator, err := infisical.NewUniversalAuthenticator(infisical.UniversalAuthConfig{
		BaseURL: config.InfisicalBaseURL, ClientID: config.InfisicalUniversalClientID,
		ClientSecret: config.InfisicalUniversalClientSecret, HTTPClient: client, Now: time.Now,
	})
	if err != nil {
		return connectionbinding.ResolverSet{}, err
	}
	resolver, err := infisical.NewResolver(infisical.Config{
		BaseURL: config.InfisicalBaseURL, HTTPClient: client,
		Authenticator: authenticator, Now: time.Now, AllowedScopes: allowed,
	})
	if err != nil {
		return connectionbinding.ResolverSet{}, err
	}
	return connectionbinding.ResolverSet{Infisical: resolver}, nil
}

func (m *Module) TargetCredentialResolver(
	selection connectionbinding.ResolverSelection,
	development connectionbinding.CredentialResolver,
) (connectionbinding.CredentialResolver, error) {
	if m == nil {
		return nil, connectionbinding.ErrProviderUnavailable
	}
	resolvers := m.targetResolvers
	resolvers.Environment = development
	return connectionbinding.SelectResolver(selection, resolvers)
}
