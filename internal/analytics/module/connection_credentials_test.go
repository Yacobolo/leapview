package module

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/flidai/leapview/internal/analytics/connectionbinding"
)

func TestBuildTargetResolversComposesOnlyTheConfiguredInfisicalAuthority(t *testing.T) {
	resolvers, err := buildTargetResolvers(TargetCredentialConfig{
		InfisicalBaseURL:               "https://infisical.example.com",
		InfisicalUniversalClientID:     "machine-client",
		InfisicalUniversalClientSecret: "bootstrap-secret",
		InfisicalAllowedScopes:         `[{"projectId":"project-1","environment":"prod","secretPathPrefix":"/leapview"}]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolvers.Infisical == nil || resolvers.Environment != nil {
		t.Fatalf("resolver set = %#v", resolvers)
	}
	module := &Module{targetResolvers: resolvers}
	selection, err := connectionbinding.NewResolverSelection(connectionbinding.ResolverSelectionInput{
		TargetID: "target-prod", Environment: "prod",
		TargetClass: connectionbinding.TargetProduction, Kind: connectionbinding.ResolverInfisical,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := module.TargetCredentialResolver(selection, nil); err != nil {
		t.Fatal(err)
	}
}

func TestBuildTargetResolversRejectsPartialOrMalformedConfigurationWithoutDisclosure(t *testing.T) {
	for _, config := range []TargetCredentialConfig{
		{InfisicalBaseURL: "https://infisical.example.com"},
		{
			InfisicalBaseURL:               "https://infisical.example.com",
			InfisicalUniversalClientID:     "machine-client",
			InfisicalUniversalClientSecret: "bootstrap-secret",
			InfisicalAllowedScopes:         "not-json",
		},
	} {
		_, err := buildTargetResolvers(config)
		if !errors.Is(err, connectionbinding.ErrInvalidBinding) ||
			strings.Contains(err.Error(), "bootstrap-secret") {
			t.Fatalf("buildTargetResolvers() error = %v", err)
		}
	}
}

func TestTargetCredentialConfigExcludesBootstrapSecretFromSerializationAndFormatting(t *testing.T) {
	config := TargetCredentialConfig{
		InfisicalBaseURL:               "https://infisical.example.com",
		InfisicalUniversalClientSecret: "bootstrap-secret",
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	for _, rendered := range []string{string(encoded), config.String(), config.GoString()} {
		if strings.Contains(rendered, "bootstrap-secret") {
			t.Fatalf("target credential config disclosed secret: %s", rendered)
		}
	}
}
