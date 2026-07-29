package module

import (
	"github.com/flidai/leapview/internal/access"
	"github.com/flidai/leapview/internal/access/httpauth"
)

type Repository = access.Repository
type DataAuthorizationService = access.DataAuthorizationService
type APICredential = access.APICredential
type Privilege = access.Privilege
type ObjectRef = access.ObjectRef
type ObjectResolver = httpauth.ObjectResolver
type AuditEventInput = access.AuditEventInput

const (
	PrivilegeViewItem = access.PrivilegeViewItem
	PrivilegeDeploy   = access.PrivilegeDeploy
)

func ParsePrivilege(value string) (Privilege, bool) {
	return access.ParsePrivilege(value)
}

func PlatformObject() ObjectRef {
	return access.PlatformObject()
}

func WorkspaceObject(workspaceID string) ObjectRef {
	return access.WorkspaceObject(workspaceID)
}

func AgentAPICredential(principalID, workspaceID string, privileges []string) APICredential {
	values := make([]access.Privilege, 0, len(privileges))
	for _, privilege := range privileges {
		values = append(values, access.Privilege(privilege))
	}
	return access.APICredential{
		Principal: access.Principal{ID: principalID},
		Token: access.APIToken{
			ID: "agent", PrincipalID: principalID,
			WorkspaceID: workspaceID, Privileges: values,
		},
	}
}
