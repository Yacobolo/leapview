package workspace

import "github.com/Yacobolo/libredash/internal/agenttools"

func IsKnownAgentTool(name string) bool {
	return agenttools.IsKnownTool(name)
}

func KnownAgentToolNames() []string {
	return agenttools.ToolNames()
}

func DefaultAgentPolicy() AgentPolicy {
	return AgentPolicy{Enabled: true}
}
