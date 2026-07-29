package module

import (
	"net/http"
	"testing"

	webpage "github.com/flidai/leapview/internal/platform/web/page"
)

func TestBuildConstructsOwnedHTTPHandler(t *testing.T) {
	module, err := Build(t.Context(), Config{
		Layout: func(*http.Request) webpage.Provider {
			return func(webpage.Context) webpage.Layout {
				return webpage.Layout{Presentation: webpage.Presentation{ProductName: "Application"}}
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := module.HTTP().Layout(nil)(webpage.Context{}).Presentation.ProductName; got != "Application" {
		t.Fatalf("product name = %q", got)
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
