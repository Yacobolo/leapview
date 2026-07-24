// Package api defines access-owned HTTP wire contracts.
package api

type RoleResponse struct {
	Name       string   `json:"name"`
	Privileges []string `json:"privileges"`
}

type RoleBindingResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	SubjectType string `json:"subjectType"`
	SubjectID   string `json:"subjectId"`
	PrincipalID string `json:"principalId"`
	GroupID     string `json:"groupId,omitempty"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	GroupName   string `json:"groupName,omitempty"`
	Role        string `json:"role"`
	CreatedAt   string `json:"createdAt"`
}
