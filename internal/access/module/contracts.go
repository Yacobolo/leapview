package module

import (
	"github.com/flidai/leapview/internal/access"
	"github.com/flidai/leapview/internal/access/httpauth"
)

type Repository = access.Repository
type DataAuthorizationService = access.DataAuthorizationService
type APICredential = access.APICredential
type CredentialEvidence = access.CredentialEvidence
type Privilege = access.Privilege
type ObjectRef = access.ObjectRef
type ObjectResolver = httpauth.ObjectResolver
type AuditEventInput = access.AuditEventInput

const (
	PrivilegeViewItem                 = access.PrivilegeViewItem
	PrivilegeDeploy                   = access.PrivilegeDeploy
	PrivilegeAuthorProject            = access.PrivilegeAuthorProject
	PrivilegePublishRelease           = access.PrivilegePublishRelease
	PrivilegeReviewCandidate          = access.PrivilegeReviewCandidate
	PrivilegeRequestDeployment        = access.PrivilegeRequestDeployment
	PrivilegeApproveDeployment        = access.PrivilegeApproveDeployment
	PrivilegeActivateDeployment       = access.PrivilegeActivateDeployment
	PrivilegeVerifyDeployment         = access.PrivilegeVerifyDeployment
	PrivilegeRollbackDeployment       = access.PrivilegeRollbackDeployment
	PrivilegePreviewData              = access.PrivilegePreviewData
	PrivilegeTestDataPolicy           = access.PrivilegeTestDataPolicy
	PrivilegeManageConnectionMetadata = access.PrivilegeManageConnectionMetadata
	PrivilegeTestConnection           = access.PrivilegeTestConnection
	PrivilegeViewConnectionHealth     = access.PrivilegeViewConnectionHealth
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

func ProjectEnvironmentObject(projectID, environment string) ObjectRef {
	return access.ProjectEnvironmentObject(projectID, environment)
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
