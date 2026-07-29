package cli

import (
	"context"

	agentcli "github.com/flidai/leapview/internal/agent/cli"
	agenttools "github.com/flidai/leapview/internal/agent/tools"
	apiaggregate "github.com/flidai/leapview/internal/app/api/aggregate"
	"github.com/spf13/cobra"
)

func agentCommand(ctx context.Context, _ *rootOptions) *cobra.Command {
	return agentcli.Command(ctx, agentcli.Dependencies{
		Client:     capabilityAPIClient{},
		Operations: cliAgentAPIGenOperations,
	})
}

func cliAgentAPIGenOperations() []agenttools.APIGenOperation {
	generated := apiaggregate.GetAPIGenOperationContracts()
	contracts := make(map[string]agenttools.OperationContract, len(generated))
	for operationID, contract := range generated {
		contracts[operationID] = agenttools.OperationContract{
			OperationID: contract.OperationID, Method: contract.Method, Path: contract.Path,
			Protected: contract.Protected, AuthzMode: contract.AuthzMode, Manual: contract.Manual,
			Extensions: contract.Extensions,
		}
	}
	return agenttools.BuildAPIGenOperations(contracts, apiaggregate.GetAPIGenToolContracts())
}
