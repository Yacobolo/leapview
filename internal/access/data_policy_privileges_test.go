package access

import (
	"slices"
	"testing"
)

func TestTestDataPolicyPrivilegeIsReservedForAdministrativeRoles(t *testing.T) {
	if parsed, ok := ParsePrivilege(string(PrivilegeTestDataPolicy)); !ok || parsed != PrivilegeTestDataPolicy {
		t.Fatalf("ParsePrivilege(%q) = %q, %v", PrivilegeTestDataPolicy, parsed, ok)
	}
	if !slices.Contains(KnownPrivileges(), PrivilegeTestDataPolicy) {
		t.Fatalf("KnownPrivileges() omits %s", PrivilegeTestDataPolicy)
	}
	for _, role := range DefaultRoles() {
		hasPrivilege := slices.Contains(role.Privileges, PrivilegeTestDataPolicy)
		switch role.Name {
		case RoleOwner, RoleAdmin, RolePlatformAdmin:
			if !hasPrivilege {
				t.Fatalf("administrative role %q lacks %s", role.Name, PrivilegeTestDataPolicy)
			}
		default:
			if hasPrivilege {
				t.Fatalf("non-administrative role %q received %s", role.Name, PrivilegeTestDataPolicy)
			}
		}
	}
}
