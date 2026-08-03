package cli

import (
	"fmt"
	"net/url"

	apiaggregate "github.com/flidai/leapview/internal/app/api/aggregate"
	"github.com/flidai/leapview/internal/app/api/clienttransport"
	apigencli "github.com/flidai/leapview/internal/app/cli/gen"
)

func apiOperationURL(target, operationID string, pathParams map[string]string, query url.Values) (string, error) {
	path, ok := generatedCLIPath(operationID)
	if !ok {
		contract, contractOK := apiaggregate.GetAPIGenOperationContract(operationID)
		if !contractOK {
			return "", fmt.Errorf("unknown API operation %q", operationID)
		}
		path = contract.Path
	}
	return clienttransport.RequestURL(target, path, pathParams, query)
}

func generatedCLIPath(operationID string) (string, bool) {
	for _, spec := range apigencli.APIGeneratedCommandSpecs {
		if spec.OperationID == operationID {
			return spec.Path, true
		}
	}
	return "", false
}
