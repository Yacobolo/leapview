package app

import (
	"database/sql"
	"testing"
	"time"

	agentmodule "github.com/Yacobolo/leapview/internal/agent/module"
	deploymentmodule "github.com/Yacobolo/leapview/internal/deployment/module"
	workspacemodule "github.com/Yacobolo/leapview/internal/workspace/module"
)

func TestRuntimeRouterReleasesConstructionInputs(t *testing.T) {
	router := &runtimeRouter{
		construction: &capabilityConstruction{
			adminDatabase:        &sql.DB{},
			workspacePersistence: &workspacemodule.Persistence{},
			agent:                &agentmodule.Service{},
			duckLakeCatalogPath:  "catalog.ducklake",
			duckLakeDataPath:     "data",
			jobLeaseTimeout:      time.Minute,
			deploymentConfig:     deploymentmodule.Config{InstanceEnvironment: "test"},
			publicURL:            "https://example.test",
		},
	}

	router.releaseConstructionInputs()

	if router.construction != nil {
		t.Fatal("capability construction inputs were retained after assembly")
	}
}
