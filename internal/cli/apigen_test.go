package cli

import (
	"net/url"
	"testing"

	cligen "github.com/Yacobolo/libredash/internal/cli/gen"
)

func TestAPIGenOperationURLUsesGeneratedContracts(t *testing.T) {
	u, err := apiOperationURL("https://libredash.example/", "rollbackDeployment", map[string]string{"deployment": "dep 1"}, nil)
	if err != nil {
		t.Fatalf("operation URL: %v", err)
	}
	if u != "https://libredash.example/api/deployments/dep%201/rollback" {
		t.Fatalf("url = %q", u)
	}

	query := url.Values{"workspace": []string{"demo"}}
	u, err = apiOperationURL("https://libredash.example", "listDeployments", nil, query)
	if err != nil {
		t.Fatalf("operation URL: %v", err)
	}
	if u != "https://libredash.example/api/deployments?workspace=demo" {
		t.Fatalf("url = %q", u)
	}
}

func TestGeneratedCLIRegistryContainsCompatibilityCommands(t *testing.T) {
	commands := map[string]bool{}
	for _, spec := range cligen.APIGeneratedCommandSpecs {
		commands[spec.OperationID] = true
	}
	for _, operationID := range []string{"listDeployments", "rollbackDeployment", "listAgentConversations"} {
		if !commands[operationID] {
			t.Fatalf("generated CLI registry missing %s", operationID)
		}
	}
}
