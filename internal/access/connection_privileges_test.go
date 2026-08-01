package access

import (
	"slices"
	"testing"
)

func TestConnectionOperatorRoleIsLeastPrivilege(t *testing.T) {
	operator, ok := roleByName(DefaultRoles(), RoleConnectionOperator)
	if !ok {
		t.Fatal("connection_operator role is missing")
	}
	want := []Privilege{
		PrivilegeManageConnectionMetadata,
		PrivilegeTestConnection,
		PrivilegeViewConnectionHealth,
	}
	if !slices.Equal(operator.Privileges, want) {
		t.Fatalf("connection operator privileges = %#v, want %#v", operator.Privileges, want)
	}
	for _, forbidden := range []Privilege{
		PrivilegeQueryData, PrivilegePreviewData, PrivilegeDeploy,
		PrivilegeActivateDeployment, PrivilegeManagePublications,
	} {
		if slices.Contains(operator.Privileges, forbidden) {
			t.Fatalf("connection operator received %s", forbidden)
		}
	}
	for _, privilege := range want {
		if _, ok := ParsePrivilege(string(privilege)); !ok || !slices.Contains(KnownPrivileges(), privilege) {
			t.Fatalf("connection privilege %s is not registered", privilege)
		}
	}
}

func roleByName(roles []Role, name string) (Role, bool) {
	for _, role := range roles {
		if role.Name == name {
			return role, true
		}
	}
	return Role{}, false
}
