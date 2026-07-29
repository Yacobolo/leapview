package connectionbinding

import (
	"context"
	"errors"
	"testing"
)

func TestSelectResolverNeverFallsBackAfterAuthoritativeProviderDenial(t *testing.T) {
	selection, err := NewResolverSelection(ResolverSelectionInput{
		TargetID: "target-prod", Environment: "prod", TargetClass: TargetProduction, Kind: ResolverInfisical,
	})
	if err != nil {
		t.Fatal(err)
	}
	authoritative := &countingCredentialResolver{err: ErrCredentialDenied}
	development := &countingCredentialResolver{}
	resolver, err := SelectResolver(selection, ResolverSet{Infisical: authoritative, Environment: development})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Resolve(context.Background(), CredentialReference{}); !errors.Is(err, ErrCredentialDenied) {
		t.Fatalf("Resolve() error = %v", err)
	}
	if authoritative.calls != 1 || development.calls != 0 {
		t.Fatalf("authoritative calls=%d development calls=%d", authoritative.calls, development.calls)
	}
}

func TestSelectResolverRequiresTheExplicitlySelectedProvider(t *testing.T) {
	selection, err := NewResolverSelection(ResolverSelectionInput{
		TargetID: "target-prod", Environment: "prod", TargetClass: TargetProduction, Kind: ResolverInfisical,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SelectResolver(selection, ResolverSet{Environment: &countingCredentialResolver{}}); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("SelectResolver() error = %v", err)
	}
}

type countingCredentialResolver struct {
	calls int
	err   error
}

func (resolver *countingCredentialResolver) Resolve(context.Context, CredentialReference) (CredentialSnapshot, error) {
	resolver.calls++
	return CredentialSnapshot{}, resolver.err
}
