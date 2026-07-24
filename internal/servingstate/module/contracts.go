package module

import "github.com/Yacobolo/leapview/internal/servingstate"

type ID = servingstate.ID
type WorkspaceID = servingstate.WorkspaceID
type Environment = servingstate.Environment
type PreparedRuntime = servingstate.PreparedRuntime
type ActiveScope = servingstate.ActiveScope

const DefaultEnvironment = servingstate.DefaultEnvironment

func NormalizeEnvironment(environment Environment) Environment {
	return servingstate.NormalizeEnvironment(environment)
}
