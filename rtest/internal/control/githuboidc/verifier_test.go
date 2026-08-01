package githuboidc_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flidai/leapview/rtest/internal/control/githuboidc"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

func TestVerifierChecksOIDCAndExtractsImmutableGitHubClaims(t *testing.T) {
	issuer, sign := testIssuer(t)
	verifier, err := githuboidc.New(context.Background(), issuer, "https://rtest.example")
	if err != nil {
		t.Fatal(err)
	}
	token := sign(t, map[string]any{
		"iss": issuer, "aud": "https://rtest.example", "sub": "repo:flidai/leapview:environment:rtest",
		"iat": time.Now().Add(-time.Minute).Unix(), "nbf": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(5 * time.Minute).Unix(),
		"repository_owner_id": "100", "repository_id": "200", "repository": "flidai/leapview",
		"workflow_ref": "flidai/leapview/.github/workflows/ci.yml@refs/heads/main", "ref": "refs/heads/main",
		"environment": "rtest", "event_name": "workflow_dispatch",
	})
	claims, err := verifier.Verify(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.RepositoryOwnerID != "100" || claims.RepositoryID != "200" || claims.WorkflowRef == "" || claims.EventName != "workflow_dispatch" {
		t.Fatalf("claims = %#v", claims)
	}
}

func TestVerifierRejectsWrongAudienceAndMissingImmutableIDs(t *testing.T) {
	issuer, sign := testIssuer(t)
	verifier, err := githuboidc.New(context.Background(), issuer, "https://rtest.example")
	if err != nil {
		t.Fatal(err)
	}
	base := map[string]any{
		"iss": issuer, "aud": "wrong", "sub": "repo:flidai/leapview:ref:refs/heads/main",
		"iat": time.Now().Add(-time.Minute).Unix(), "nbf": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(5 * time.Minute).Unix(),
		"repository_owner_id": "100", "repository_id": "200", "workflow_ref": "workflow", "ref": "refs/heads/main", "event_name": "push",
	}
	if _, err := verifier.Verify(context.Background(), sign(t, base)); err == nil {
		t.Fatal("wrong audience was accepted")
	}
	base["aud"] = "https://rtest.example"
	delete(base, "repository_id")
	if _, err := verifier.Verify(context.Background(), sign(t, base)); err == nil {
		t.Fatal("missing repository ID was accepted")
	}
}

func testIssuer(t *testing.T) (string, func(*testing.T, map[string]any) string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	jwk := jose.JSONWebKey{Key: &key.PublicKey, KeyID: "test", Algorithm: string(jose.RS256), Use: "sig"}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(response).Encode(map[string]any{
				"issuer": server.URL, "jwks_uri": server.URL + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/keys":
			_ = json.NewEncoder(response).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}})
		default:
			http.NotFound(response, request)
		}
	}))
	t.Cleanup(server.Close)
	return server.URL, func(t *testing.T, claims map[string]any) string {
		t.Helper()
		signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, &jose.SignerOptions{ExtraHeaders: map[jose.HeaderKey]any{jose.HeaderKey("kid"): "test"}})
		if err != nil {
			t.Fatal(err)
		}
		token, err := jwt.Signed(signer).Claims(claims).Serialize()
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
}
