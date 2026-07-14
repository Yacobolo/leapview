package access

import (
	"slices"
	"testing"
)

func TestManagedDataPrivilegesAndDataDeployerRole(t *testing.T) {
	wantPrivileges := []Privilege{PrivilegeViewData, PrivilegeIngestData, PrivilegeActivateData}
	known := KnownPrivileges()
	for _, privilege := range wantPrivileges {
		if !slices.Contains(known, privilege) {
			t.Fatalf("KnownPrivileges() = %#v, missing %s", known, privilege)
		}
	}

	var deployer *Role
	for _, role := range DefaultRoles() {
		if role.Name == RoleDataDeployer {
			copy := role
			deployer = &copy
			break
		}
	}
	if deployer == nil {
		t.Fatal("data_deployer role is missing")
	}
	if !slices.Equal(deployer.Privileges, wantPrivileges) {
		t.Fatalf("data_deployer privileges = %#v, want %#v", deployer.Privileges, wantPrivileges)
	}
}
