package access

func TokenAllows(token APIToken, workspaceID string, privilege Privilege) bool {
	if token.WorkspaceID != "" && token.WorkspaceID != workspaceID {
		return false
	}
	if token.Privileges == nil {
		return true
	}
	for _, allowed := range token.Privileges {
		if allowed == privilege {
			return true
		}
	}
	return false
}
