package module

import (
	"context"

	"github.com/Yacobolo/leapview/internal/access"
	queryauthz "github.com/Yacobolo/leapview/internal/dashboard/queryauthz"
	"github.com/Yacobolo/leapview/internal/dashboard/queryruntime"
)

type QueryPrincipal struct {
	ID        string
	DevBypass bool
}

type QueryAuthorizationConfig struct {
	Repository            access.DataAuthorizationService
	DefaultWorkspaceID    string
	PrincipalFromContext  func(context.Context) (QueryPrincipal, bool)
	CredentialFromContext func(context.Context) (access.APICredential, bool)
	TokenAllows           func(access.APIToken, string, access.Privilege) bool
}

func WithQueryAuthorization(metrics queryruntime.Metrics, config QueryAuthorizationConfig) queryruntime.Metrics {
	if metrics == nil || config.Repository == nil {
		return metrics
	}
	return queryauthz.New(metrics, queryauthz.Options{
		Repo: config.Repository, DefaultWorkspaceID: config.DefaultWorkspaceID,
		PrincipalFromContext: func(ctx context.Context) (queryauthz.Principal, bool) {
			if config.PrincipalFromContext == nil {
				return queryauthz.Principal{}, false
			}
			principal, ok := config.PrincipalFromContext(ctx)
			return queryauthz.Principal{ID: principal.ID, DevBypass: principal.DevBypass}, ok
		},
		CredentialFromContext: config.CredentialFromContext,
		TokenAllows:           config.TokenAllows,
	})
}
