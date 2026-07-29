package ui

import (
	"html"
	"strings"
	"testing"

	webpage "github.com/flidai/leapview/internal/platform/web/page"
	"github.com/flidai/leapview/internal/platform/web/staticasset"
)

func TestDeviceAuthorizationPageRendersCSRFProtectedDecision(t *testing.T) {
	var output strings.Builder
	if err := DeviceAuthorizationPage(DeviceAuthorizationPageOptions{
		UserCode: "<ABCD-EFGH>", CSRFToken: "csrf-token",
		Presentation: webpage.Presentation{ProductName: "LeapView", FaviconPath: "/static/favicon.svg"},
		Assets:       staticasset.Resolver{},
	}).Render(&output); err != nil {
		t.Fatal(err)
	}
	rendered := html.UnescapeString(output.String())
	for _, expected := range []string{
		"<title>Authorize CLI · LeapView</title>",
		`action="/device"`,
		`name="gorilla.csrf.Token" value="csrf-token"`,
		`name="user_code" value="<ABCD-EFGH>"`,
		`name="decision" value="deny"`,
		`name="decision" value="approve"`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Errorf("document does not contain %q:\n%s", expected, rendered)
		}
	}
	if strings.Contains(output.String(), `value="<ABCD-EFGH>"`) {
		t.Fatal("user code was not HTML-escaped")
	}
}

func TestDeviceAuthorizationResultPageDoesNotExposeCredentials(t *testing.T) {
	var output strings.Builder
	if err := DeviceAuthorizationResultPage(true, webpage.Presentation{ProductName: "LeapView"}, staticasset.Resolver{}).Render(&output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Return to your terminal") ||
		strings.Contains(strings.ToLower(output.String()), "access token") ||
		strings.Contains(strings.ToLower(output.String()), "refresh token") {
		t.Fatalf("result page = %s", output.String())
	}
}
