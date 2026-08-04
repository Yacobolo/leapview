package docs

import (
	"strings"
	"testing"
)

func TestDesktopDocumentationCoversTheConsumerLifecycle(t *testing.T) {
	required := map[string][]string{
		"articles/desktop/overview.md": {
			"end-user client",
			"deployed LeapView instance",
			"does not host",
			"/download",
		},
		"articles/desktop/install.md": {
			"signed and notarized DMG",
			"per-user Squirrel",
			"signed LeapView APT repository",
			"Uninstall",
			"source code",
		},
		"articles/desktop/connect-profiles.md": {
			"canonical HTTPS",
			"immutable instance identity",
			"Remove",
			"Disconnect",
			"credentials",
		},
		"articles/desktop/authentication.md": {
			"system browser",
			"PKCE",
			"127.0.0.1",
			"HttpOnly",
			"administrator revocation",
		},
		"articles/desktop/updates.md": {
			"Check for Updates",
			"Restart now",
			"Later",
			"APT",
			"does not downgrade",
		},
		"articles/desktop/support.md": {
			"Diagnostics",
			"DNS",
			"proxy",
			"certificate",
			"session expired",
		},
		"articles/desktop/security.md": {
			"untrusted",
			"operating-system trust store",
			"client certificates",
			"machine-wide",
			"private update mirrors",
		},
		"articles/desktop/release-verification.md": {
			"SHA-256",
			"SBOM",
			"provenance",
			"code signature",
			"withdrawn",
		},
	}
	for path, phrases := range required {
		contents, err := Files.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		document := string(contents)
		for _, phrase := range phrases {
			if !strings.Contains(strings.ToLower(document), strings.ToLower(phrase)) {
				t.Errorf("%s does not cover %q", path, phrase)
			}
		}
	}
}
