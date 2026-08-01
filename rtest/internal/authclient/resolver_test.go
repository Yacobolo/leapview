package authclient_test

import (
	"context"
	"errors"
	"testing"

	"github.com/flidai/leapview/rtest/internal/authclient"
)

func TestResolutionOrderIsExplicitEnvironmentKeyringThenOIDC(t *testing.T) {
	store := &memoryKeyring{token: "stored"}
	t.Setenv("RTEST_TOKEN", "environment")
	token, source, err := authclient.Resolve(context.Background(), authclient.ResolveOptions{
		ExplicitToken: "explicit", ServiceURL: "https://rtest.example", Keyring: store,
		OIDC: func(context.Context) (string, error) { return "oidc", nil },
	})
	if err != nil || token != "explicit" || source != authclient.SourceExplicit {
		t.Fatalf("token=%q source=%q err=%v", token, source, err)
	}
	t.Setenv("RTEST_TOKEN", "")
	token, source, err = authclient.Resolve(context.Background(), authclient.ResolveOptions{
		ServiceURL: "https://rtest.example", Keyring: store, OIDC: func(context.Context) (string, error) { return "oidc", nil },
	})
	if err != nil || token != "stored" || source != authclient.SourceKeyring {
		t.Fatalf("token=%q source=%q err=%v", token, source, err)
	}
	store.err = errors.New("missing")
	token, source, err = authclient.Resolve(context.Background(), authclient.ResolveOptions{
		ServiceURL: "https://rtest.example", Keyring: store, OIDC: func(context.Context) (string, error) { return "oidc", nil },
	})
	if err != nil || token != "oidc" || source != authclient.SourceOIDC {
		t.Fatalf("token=%q source=%q err=%v", token, source, err)
	}
}

type memoryKeyring struct {
	token string
	err   error
}

func (m *memoryKeyring) Get(string, string) (string, error) { return m.token, m.err }
func (m *memoryKeyring) Set(string, string, string) error   { return nil }
func (m *memoryKeyring) Delete(string, string) error        { return nil }
