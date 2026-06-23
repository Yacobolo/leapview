package api

type WorkspaceResponse struct {
	ID                 string `json:"id"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	ActiveDeploymentID string `json:"activeDeploymentId,omitempty"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

type AssetResponse struct {
	ID           string         `json:"id"`
	WorkspaceID  string         `json:"workspaceId"`
	DeploymentID string         `json:"deploymentId"`
	Type         string         `json:"type"`
	Key          string         `json:"key"`
	ParentID     string         `json:"parentId,omitempty"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	Meta         map[string]any `json:"meta,omitempty"`
	Href         string         `json:"href,omitempty"`
}

type AssetEdgeResponse struct {
	ID           string `json:"id"`
	WorkspaceID  string `json:"workspaceId"`
	DeploymentID string `json:"deploymentId"`
	FromAssetID  string `json:"fromAssetId"`
	ToAssetID    string `json:"toAssetId"`
	Type         string `json:"type"`
}

type RoleResponse struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

type RoleBindingResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	PrincipalID string `json:"principalId"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	CreatedAt   string `json:"createdAt"`
}

type RoleBindingUpsertRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

type WorkspaceAccessResponse struct {
	Workspace WorkspaceResponse     `json:"workspace"`
	Roles     []RoleResponse        `json:"roles"`
	Bindings  []RoleBindingResponse `json:"bindings"`
	CanManage bool                  `json:"canManage"`
	Status    WorkspaceAccessStatus `json:"status"`
}

type WorkspaceAccessStatus struct {
	Loading bool   `json:"loading"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

type WorkspaceAccessCommand struct {
	Email       string `json:"email"`
	Role        string `json:"role"`
	PrincipalID string `json:"principalId"`
}
