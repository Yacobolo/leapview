package ui

import (
	uisignals "github.com/Yacobolo/leapview/internal/dashboard/ui/signals"
	visualizationir "github.com/Yacobolo/leapview/internal/dashboard/visualization/ir"
)

// AgentBootstrap is agent-owned state projected into dashboard browser
// contracts at the application composition boundary.
type AgentBootstrap struct {
	Agent   uisignals.ChatSignal
	Visuals map[string]visualizationir.VisualizationEnvelope
}
