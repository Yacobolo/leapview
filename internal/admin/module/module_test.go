package module

import (
	"net/http"
	"testing"
)

func TestBuildConstructsOwnedHTTPHandler(t *testing.T) {
	module, err := Build(t.Context(), Config{
		CurrentRoleLabel: func(*http.Request) string { return "Platform access" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := module.HTTP().CurrentRoleLabel(nil); got != "Platform access" {
		t.Fatalf("role label = %q", got)
	}
}

func TestRoleLabelDistinguishesLocalAndConfiguredAccess(t *testing.T) {
	if got := RoleLabel(false, Principal{}, false); got != "Local platform" {
		t.Fatalf("local label = %q", got)
	}
	if got := RoleLabel(true, Principal{DevBypass: true}, true); got != "Platform admin" {
		t.Fatalf("admin label = %q", got)
	}
	if got := RoleLabel(true, Principal{}, true); got != "Platform access" {
		t.Fatalf("access label = %q", got)
	}
}
