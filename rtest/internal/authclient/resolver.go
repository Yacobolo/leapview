package authclient

import (
	"context"
	"errors"
	"os"
)

type Source string

const (
	SourceExplicit    Source = "explicit"
	SourceEnvironment Source = "environment"
	SourceKeyring     Source = "keyring"
	SourceOIDC        Source = "oidc"
)

type ResolveOptions struct {
	ExplicitToken string
	ServiceURL    string
	Keyring       Keyring
	OIDC          func(context.Context) (string, error)
}

func Resolve(ctx context.Context, options ResolveOptions) (string, Source, error) {
	if options.ExplicitToken != "" {
		return options.ExplicitToken, SourceExplicit, nil
	}
	if token := os.Getenv("RTEST_TOKEN"); token != "" {
		return token, SourceEnvironment, nil
	}
	var keyringErr error
	if options.Keyring != nil {
		if token, err := StoredToken(options.Keyring, options.ServiceURL); err == nil && token != "" {
			return token, SourceKeyring, nil
		} else {
			keyringErr = err
		}
	}
	if options.OIDC != nil {
		token, err := options.OIDC(ctx)
		if err == nil && token != "" {
			return token, SourceOIDC, nil
		}
		if err != nil {
			return "", "", err
		}
	}
	if keyringErr != nil {
		return "", "", keyringErr
	}
	return "", "", errors.New("no rtest credential is available; run rtest login, set RTEST_TOKEN, or use GitHub OIDC")
}
