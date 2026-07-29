package environment

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/flidai/leapview/internal/analytics/connectionbinding"
)

func TestResolverRequiresExplicitDevelopmentSelectionAndAllowedVariable(t *testing.T) {
	selection, err := connectionbinding.NewResolverSelection(connectionbinding.ResolverSelectionInput{
		TargetID: "local-target", Environment: "dev", TargetClass: connectionbinding.TargetDevelopment,
		Kind: connectionbinding.ResolverEnvironment,
	})
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := NewResolver(Config{
		Selection: selection, AllowedVariables: []string{"LEAPVIEW_DEV_WAREHOUSE"},
		LookupEnv: func(name string) (string, bool) {
			if name != "LEAPVIEW_DEV_WAREHOUSE" {
				t.Fatalf("looked up unexpected variable %q", name)
			}
			return `{"password":"development-source-secret"}`, true
		},
		Now: func() time.Time { return time.Date(2026, 7, 29, 18, 0, 0, 0, time.UTC) },
		TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := resolver.Resolve(context.Background(), connectionbinding.CredentialReference{
		ProjectID: "local-target", Environment: "dev", SecretPath: "/", SecretKey: "LEAPVIEW_DEV_WAREHOUSE",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Destroy()
	if !strings.HasPrefix(snapshot.ProviderVersion(), "env:sha256:") {
		t.Fatalf("provider version = %q", snapshot.ProviderVersion())
	}
}

func TestResolverRejectsProductionSelectionAndOutOfScopeReferenceBeforeLookup(t *testing.T) {
	production, err := connectionbinding.NewResolverSelection(connectionbinding.ResolverSelectionInput{
		TargetID: "target-prod", Environment: "prod", TargetClass: connectionbinding.TargetProduction,
		Kind: connectionbinding.ResolverInfisical,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewResolver(Config{Selection: production, LookupEnv: func(string) (string, bool) {
		return "", false
	}, AllowedVariables: []string{"LEAPVIEW_DEV_WAREHOUSE"}, Now: time.Now, TTL: time.Minute}); !errors.Is(err, connectionbinding.ErrInvalidBinding) {
		t.Fatalf("NewResolver(production) error = %v", err)
	}

	development, err := connectionbinding.NewResolverSelection(connectionbinding.ResolverSelectionInput{
		TargetID: "local-target", Environment: "dev", TargetClass: connectionbinding.TargetDevelopment,
		Kind: connectionbinding.ResolverEnvironment,
	})
	if err != nil {
		t.Fatal(err)
	}
	lookups := 0
	resolver, err := NewResolver(Config{
		Selection: development, AllowedVariables: []string{"LEAPVIEW_DEV_WAREHOUSE"},
		LookupEnv: func(string) (string, bool) {
			lookups++
			return "", false
		},
		Now: time.Now, TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Resolve(context.Background(), connectionbinding.CredentialReference{
		ProjectID: "other-target", Environment: "dev", SecretPath: "/", SecretKey: "LEAPVIEW_DEV_WAREHOUSE",
	}); !errors.Is(err, connectionbinding.ErrCredentialDenied) {
		t.Fatalf("Resolve(out of scope) error = %v", err)
	}
	if lookups != 0 {
		t.Fatalf("environment lookups = %d", lookups)
	}
}
