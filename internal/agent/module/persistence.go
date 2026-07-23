package module

import (
	"database/sql"

	"github.com/Yacobolo/leapview/internal/agent"
	agentsqlite "github.com/Yacobolo/leapview/internal/agent/sqlite"
	jobsqlite "github.com/Yacobolo/leapview/internal/platform/jobs/sqlite"
)

func newRepository(database *sql.DB) agent.Repository {
	return agentsqlite.NewRepositoryWithEvents(database, jobsqlite.NewRepository(database))
}
