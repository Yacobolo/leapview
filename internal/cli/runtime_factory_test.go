package cli

import (
	"strings"
	"testing"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	"github.com/Yacobolo/libredash/internal/runtimehost"
	servingstate "github.com/Yacobolo/libredash/internal/servingstate"
	"github.com/Yacobolo/libredash/internal/workspace"
)

func TestRuntimeDataDirUsesConfiguredDataDir(t *testing.T) {
	input := runtimehost.RuntimeInput{
		State:   servingstate.State{WorkspaceID: "movielens"},
		DataDir: ".data/olist",
	}
	if got := runtimeDataDir(input, ".data/fallback"); got != ".data/olist" {
		t.Fatalf("runtimeDataDir = %q, want configured data dir", got)
	}
	if got := runtimeDataDir(runtimehost.RuntimeInput{}, ".data/fallback"); got != ".data/fallback" {
		t.Fatalf("runtimeDataDir = %q, want configured fallback", got)
	}
}

func TestBindManagedDataRootsUsesTrustedRuntimeResolution(t *testing.T) {
	definition := &workspace.Definition{Models: map[string]*semanticmodel.Model{
		"sales": {Connections: map[string]semanticmodel.Connection{
			"olist": {Kind: "managed"},
			"local": {Kind: "local", Root: "fixtures"},
		}},
	}}
	resolution := runtimehost.ManagedDataResolution{
		RevisionID: "sha256:" + strings.Repeat("a", 64),
		Roots:      map[string]string{"olist": "/managed/olist/revision"},
	}
	if err := bindManagedDataRoots(definition, resolution); err != nil {
		t.Fatal(err)
	}
	if got := definition.Models["sales"].Connections["olist"].Root; got != "/managed/olist/revision" {
		t.Fatalf("olist root = %q", got)
	}
	if got := definition.Models["sales"].Connections["local"].Root; got != "fixtures" {
		t.Fatalf("local root = %q", got)
	}
}

func TestBindManagedDataRootsRequiresEveryManagedConnection(t *testing.T) {
	definition := &workspace.Definition{Models: map[string]*semanticmodel.Model{
		"sales": {Connections: map[string]semanticmodel.Connection{"olist": {Kind: "managed"}}},
	}}
	err := bindManagedDataRoots(definition, runtimehost.ManagedDataResolution{})
	if err == nil || !strings.Contains(err.Error(), "olist") {
		t.Fatalf("bind error = %v, want missing olist revision", err)
	}
}
