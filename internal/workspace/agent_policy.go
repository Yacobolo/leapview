package workspace

import agenttools "github.com/Yacobolo/libredash/internal/agent/tools"

func IsKnownAgentTool(name string) bool {
	return agenttools.IsKnownTool(name)
}

func KnownAgentToolNames() []string {
	return agenttools.ToolNames()
}

func DefaultAgentPolicy() AgentPolicy {
	return AgentPolicy{Enabled: true}
}
