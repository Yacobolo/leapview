package runtimefactory

import (
	"strings"
	"testing"

	semanticmodel "github.com/Yacobolo/leapview/internal/analytics/model"
	"github.com/Yacobolo/leapview/internal/project/manifest"
	"github.com/Yacobolo/leapview/internal/runtimehost"
)

func TestBindManagedDataRootsUsesTrustedRuntimeResolution(t *testing.T) {
	definition := &manifest.Workspace{Models: map[string]*semanticmodel.Model{
		"sales": {Connections: map[string]semanticmodel.Connection{
			"olist": {Kind: "managed"},
			"cloud": {Kind: "s3", Scope: "s3://warehouse/"},
		}},
	}}
	resolution := runtimehost.ManagedDataResolution{
		RevisionID: "sha256:" + strings.Repeat("a", 64),
		Roots:      map[string]string{"olist": "/managed/olist/revision"},
	}
	if err := bindManagedDataRoots(definition, resolution.Roots); err != nil {
		t.Fatal(err)
	}
	if got := definition.Models["sales"].Connections["olist"].Root; got != "/managed/olist/revision" {
		t.Fatalf("olist root = %q", got)
	}
	if got := definition.Models["sales"].Connections["cloud"].Scope; got != "s3://warehouse/" {
		t.Fatalf("cloud scope = %q", got)
	}
}

func TestBindManagedDataRootsRequiresEveryManagedConnection(t *testing.T) {
	definition := &manifest.Workspace{Models: map[string]*semanticmodel.Model{
		"sales": {Connections: map[string]semanticmodel.Connection{"olist": {Kind: "managed"}}},
	}}
	err := bindManagedDataRoots(definition, nil)
	if err == nil || !strings.Contains(err.Error(), "olist") {
		t.Fatalf("bind error = %v, want missing olist revision", err)
	}
}
