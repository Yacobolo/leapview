package compiler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Yacobolo/libredash/internal/access"
	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	"github.com/Yacobolo/libredash/internal/workspace"
)

func validateProject(project Project) error {
	for connectionName, connection := range project.Connections {
		if _, err := connection.Validate(connectionName); err != nil {
			return resourceError(project.ConnectionPaths[connectionName], "connection:"+connectionName, "spec", "Connection %q %s", connectionName, err.Error())
		}
	}
	for sourceName, source := range project.Sources {
		if _, ok := project.Connections[source.Connection]; !ok {
			return resourceError(project.SourcePaths[sourceName], "source:"+sourceName, "spec.connection", "Source %q references unknown Connection %q", sourceName, source.Connection)
		}
		if source.Path != "" && source.Format == "" {
			format, ok := semanticmodel.InferFormat(source.Path)
			if !ok {
				return resourceError(project.SourcePaths[sourceName], "source:"+sourceName, "spec.format", "Source %q path %q requires format", sourceName, source.Path)
			}
			source.Format = format
		}
		if err := source.Validate(localSourceName(sourceName), project.Connections); err != nil {
			return resourceError(project.SourcePaths[sourceName], "source:"+sourceName, "spec", "Source %q %s", sourceName, err.Error())
		}
	}
	for _, workspaceProject := range project.Workspaces {
		for source := range workspaceProject.AllowedSources {
			if _, ok := project.Sources[source]; !ok {
				return resourceError(workspaceProject.Path, "workspace:"+workspaceProject.ID, "spec.uses.sources", "Workspace %q allows unknown Source %q", workspaceProject.ID, source)
			}
		}
		if len(workspaceProject.SemanticModels) == 0 {
			return resourceError(workspaceProject.Path, "workspace:"+workspaceProject.ID, "spec.semanticModels", "Workspace %q requires SemanticModel resources", workspaceProject.ID)
		}
		for tableName, table := range workspaceProject.Models {
			for _, source := range table.Sources {
				if _, ok := workspaceProject.AllowedSources[source]; !ok {
					return resourceError(workspaceProject.ModelPaths[tableName], "model_table:"+workspaceProject.ID+"."+tableName, "spec.sources", "ModelTable %q.%q reads source %q outside uses.sources", workspaceProject.ID, tableName, source)
				}
			}
			if table.Source != "" {
				if _, ok := workspaceProject.AllowedSources[table.Source]; !ok {
					return resourceError(workspaceProject.ModelPaths[tableName], "model_table:"+workspaceProject.ID+"."+tableName, "spec.source", "ModelTable %q.%q reads source %q outside uses.sources", workspaceProject.ID, tableName, table.Source)
				}
			}
			if err := validateProjectTableSources(workspaceProject.ID, tableName, workspaceProject.ModelPaths[tableName], table); err != nil {
				return err
			}
		}
		for name, dashboard := range workspaceProject.Dashboards {
			if _, ok := workspaceProject.SemanticModels[dashboard.SemanticModel]; !ok {
				return resourceError(workspaceProject.DashboardPaths[name], "dashboard:"+workspaceProject.ID+"."+name, "spec.semanticModel", "Dashboard %q.%q references unknown SemanticModel %q", workspaceProject.ID, name, dashboard.SemanticModel)
			}
		}
		if err := validateWorkspaceAccess(workspaceProject); err != nil {
			return err
		}
		if err := validateWorkspaceAgentPolicies(workspaceProject); err != nil {
			return err
		}
	}
	return nil
}

func validateWorkspaceAgentPolicies(workspaceProject *WorkspaceProject) error {
	for name, policy := range workspaceProject.AgentPolicies {
		path := workspaceProject.AgentPolicyPaths[name]
		allow := map[string]struct{}{}
		for _, tool := range policy.Tools.Allow {
			if !workspace.IsKnownAgentTool(tool) {
				return resourceError(path, "workspace_agent_policy:"+workspaceProject.ID+"."+name, "spec.tools.allow", "WorkspaceAgentPolicy %q.%q references unknown agent tool %q", workspaceProject.ID, name, tool)
			}
			allow[tool] = struct{}{}
		}
		for _, tool := range policy.Tools.Deny {
			if !workspace.IsKnownAgentTool(tool) {
				return resourceError(path, "workspace_agent_policy:"+workspaceProject.ID+"."+name, "spec.tools.deny", "WorkspaceAgentPolicy %q.%q references unknown agent tool %q", workspaceProject.ID, name, tool)
			}
			if _, ok := allow[tool]; ok {
				return resourceError(path, "workspace_agent_policy:"+workspaceProject.ID+"."+name, "spec.tools", "WorkspaceAgentPolicy %q.%q agent tool %q is both allowed and denied", workspaceProject.ID, name, tool)
			}
		}
	}
	return nil
}

func validateWorkspaceAccess(workspaceProject *WorkspaceProject) error {
	validRoles := map[string]struct{}{
		access.RoleOwner:    {},
		access.RoleAdmin:    {},
		access.RoleDeployer: {},
		access.RoleEditor:   {},
		access.RoleViewer:   {},
	}
	for name, group := range workspaceProject.AccessGroups {
		for index, member := range group.Members {
			if member.PrincipalID == "" && member.Email == "" {
				return resourceError(workspaceProject.AccessPaths["WorkspaceGroup:"+name], "workspace_group:"+workspaceProject.ID+"."+name, fmt.Sprintf("spec.members[%d]", index), "WorkspaceGroup %q.%q member requires principalId or email", workspaceProject.ID, name)
			}
		}
	}
	for name, binding := range workspaceProject.AccessRoleBindings {
		path := workspaceProject.AccessPaths["WorkspaceRoleBinding:"+name]
		if _, ok := validRoles[binding.Role]; !ok {
			return resourceError(path, "workspace_role_binding:"+workspaceProject.ID+"."+name, "spec.role", "WorkspaceRoleBinding %q.%q references unknown role %q", workspaceProject.ID, name, binding.Role)
		}
		switch binding.Subject.Kind {
		case string(access.SubjectGroup):
			if binding.Subject.Group == "" {
				return resourceError(path, "workspace_role_binding:"+workspaceProject.ID+"."+name, "spec.subject.group", "WorkspaceRoleBinding %q.%q group subject requires group", workspaceProject.ID, name)
			}
			if _, ok := workspaceProject.AccessGroups[binding.Subject.Group]; !ok {
				return resourceError(path, "workspace_role_binding:"+workspaceProject.ID+"."+name, "spec.subject.group", "WorkspaceRoleBinding %q.%q references unknown WorkspaceGroup %q", workspaceProject.ID, name, binding.Subject.Group)
			}
		case string(access.SubjectPrincipal):
			if binding.Subject.PrincipalID == "" && binding.Subject.Email == "" {
				return resourceError(path, "workspace_role_binding:"+workspaceProject.ID+"."+name, "spec.subject", "WorkspaceRoleBinding %q.%q principal subject requires principalId or email", workspaceProject.ID, name)
			}
		default:
			return resourceError(path, "workspace_role_binding:"+workspaceProject.ID+"."+name, "spec.subject.kind", "WorkspaceRoleBinding %q.%q has unsupported subject kind %q", workspaceProject.ID, name, binding.Subject.Kind)
		}
	}
	return nil
}

func validateProjectTableSources(workspaceID, tableName, path string, table semanticmodel.Table) error {
	sql := strings.TrimSpace(table.Transform.SQL)
	if sql == "" {
		sql = strings.TrimSpace(table.SQL)
	}
	if sql == "" {
		return nil
	}
	declared := append([]string{}, table.Sources...)
	if table.Source != "" {
		declared = append(declared, table.Source)
	}
	sort.Strings(declared)
	inferred, rawRefs, unqualifiedRefs := (&semanticmodel.Model{}).SQLSourceRefs(sql)
	if len(rawRefs) > 0 {
		return resourceError(path, "model_table:"+workspaceID+"."+tableName, "spec.sql", "ModelTable %q.%q SQL must reference sources through source.<name>; raw.<name> is internal", workspaceID, tableName)
	}
	if len(unqualifiedRefs) > 0 {
		return resourceError(path, "model_table:"+workspaceID+"."+tableName, "spec.sql", "ModelTable %q.%q SQL must reference sources through source.<name>; found unqualified relation %q", workspaceID, tableName, unqualifiedRefs[0])
	}
	if !sameStringList(declared, inferred) {
		return resourceError(path, "model_table:"+workspaceID+"."+tableName, "spec.sources", "ModelTable %q.%q SQL source references %v do not match declared sources %v", workspaceID, tableName, inferred, declared)
	}
	return nil
}
