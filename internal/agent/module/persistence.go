package module

import (
	"database/sql"

	"github.com/flidai/leapview/internal/agent"
	agentsqlite "github.com/flidai/leapview/internal/agent/sqlite"
	"github.com/flidai/leapview/internal/platform/jobs"
	jobsqlite "github.com/flidai/leapview/internal/platform/jobs/sqlite"
)

func newRepository(database *sql.DB, workflow jobs.WorkflowRecorder) agent.Repository {
	return agentsqlite.NewRepositoryWithWorkflow(database, jobsqlite.NewRepository(database), workflow)
}
