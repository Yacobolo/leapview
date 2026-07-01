package workspace

type AccessPolicy struct {
	Groups       map[string]WorkspaceGroup       `json:"groups,omitempty"`
	RoleBindings map[string]WorkspaceRoleBinding `json:"roleBindings,omitempty"`
}

type WorkspaceGroup struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Members     []WorkspaceGroupMember `json:"members,omitempty"`
}

type WorkspaceGroupMember struct {
	PrincipalID string `json:"principalId,omitempty"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

type WorkspaceRoleBinding struct {
	ID      string                      `json:"id"`
	Name    string                      `json:"name"`
	Role    string                      `json:"role"`
	Subject WorkspaceRoleBindingSubject `json:"subject"`
}

type WorkspaceRoleBindingSubject struct {
	Kind        string `json:"kind"`
	PrincipalID string `json:"principalId,omitempty"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Group       string `json:"group,omitempty"`
}
