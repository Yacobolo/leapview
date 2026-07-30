package access

import (
	"slices"
	"testing"
)

func TestEnterpriseAuthoringPrivilegesAreIndependent(t *testing.T) {
	want := []Privilege{
		PrivilegeAuthorProject,
		PrivilegePublishRelease,
		PrivilegeReviewCandidate,
		PrivilegeRequestDeployment,
		PrivilegeApproveDeployment,
		PrivilegeActivateDeployment,
		PrivilegeVerifyDeployment,
		PrivilegeRollbackDeployment,
	}
	known := KnownPrivileges()
	for _, privilege := range want {
		if parsed, ok := ParsePrivilege(string(privilege)); !ok || parsed != privilege {
			t.Errorf("ParsePrivilege(%q) = %q, %t", privilege, parsed, ok)
		}
		if !slices.Contains(known, privilege) {
			t.Errorf("KnownPrivileges() omits %q", privilege)
		}
	}
}

func TestEnterpriseAuthoringRolesRemainLeastPrivilege(t *testing.T) {
	want := map[string][]Privilege{
		RoleProjectAuthor:       {PrivilegeAuthorProject},
		RoleReleasePublisher:    {PrivilegePublishRelease, PrivilegeRequestDeployment},
		RoleDeploymentReviewer:  {PrivilegeReviewCandidate, PrivilegeApproveDeployment},
		RoleDeploymentActivator: {PrivilegeActivateDeployment},
		RoleDeploymentVerifier:  {PrivilegeVerifyDeployment},
		RoleRollbackOperator:    {PrivilegeRollbackDeployment},
	}
	roles := DefaultRoles()
	for name, expected := range want {
		var actual []Privilege
		for _, role := range roles {
			if role.Name == name {
				actual = role.Privileges
				break
			}
		}
		if !slices.Equal(actual, expected) {
			t.Errorf("role %q privileges = %#v, want %#v", name, actual, expected)
		}
		for _, forbidden := range []Privilege{
			PrivilegeQueryData,
			PrivilegePreviewData,
			PrivilegeManageConnectionMetadata,
			PrivilegeManageGrants,
			PrivilegeManagePlatform,
		} {
			if slices.Contains(actual, forbidden) {
				t.Errorf("role %q unexpectedly grants %q", name, forbidden)
			}
		}
	}
}
