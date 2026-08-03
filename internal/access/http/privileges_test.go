package http

import (
	"slices"
	"testing"

	"github.com/flidai/leapview/internal/access"
)

func TestGrantPrivilegeValidationCoversEveryKnownPrivilege(t *testing.T) {
	names := knownPrivileges()
	for _, privilege := range access.KnownPrivileges() {
		if !knownPrivilege(privilege) {
			t.Errorf("knownPrivilege(%q) = false", privilege)
		}
		if !slices.Contains(names, string(privilege)) {
			t.Errorf("knownPrivileges() omits %q", privilege)
		}
	}
}
