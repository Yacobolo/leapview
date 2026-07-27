package module

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Yacobolo/leapview/internal/access"
	"github.com/Yacobolo/leapview/internal/dashboard/publication"
	"github.com/Yacobolo/leapview/internal/dataquery"
)

func TestPublicationExecutionContextUsesPublicationPrincipal(t *testing.T) {
	row := publication.Publication{
		WorkspaceID: "visuals",
		Name:        "website-showcase",
		Dashboard:   "visual-showcase",
	}

	metadata := dataquery.MetadataFromContext(PublicationExecutionContext(context.Background(), row, ""))
	want := access.DashboardPublicationSubjectID("visuals", "website-showcase")
	if metadata.PrincipalID != want {
		t.Fatalf("public principal id = %q, want %q", metadata.PrincipalID, want)
	}
	if metadata.Surface != dataquery.SurfacePublicDashboard || metadata.ObjectType != "dashboard_publication" || metadata.ObjectID != "website-showcase" {
		t.Fatalf("public metadata = %#v", metadata)
	}
}

func TestEmbedWithNoAllowedOriginsDeniesFraming(t *testing.T) {
	header := http.Header{}
	SetPublicDashboardSecurityHeaders(header, "embed", nil)
	if got := header.Get("X-Frame-Options"); got != "" {
		t.Fatalf("X-Frame-Options = %q, want omitted", got)
	}
	if got := header.Get("Content-Security-Policy"); !strings.Contains(got, "frame-ancestors 'none'") {
		t.Fatalf("Content-Security-Policy = %q", got)
	}
}
