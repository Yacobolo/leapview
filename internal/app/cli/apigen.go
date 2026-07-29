package cli

import (
	"fmt"
	"net/url"
	"strings"

	apiaggregate "github.com/flidai/leapview/internal/app/api/aggregate"
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
	return apiRequestURL(target, path, pathParams, query)
}

func apiRequestURL(target, path string, pathParams map[string]string, query url.Values) (string, error) {
	for name, value := range pathParams {
		path = strings.ReplaceAll(path, "{"+name+"}", url.PathEscape(value))
	}
	if strings.Contains(path, "{") {
		return "", fmt.Errorf("unresolved API path parameter in %q", path)
	}
	u, err := url.Parse(strings.TrimRight(target, "/") + path)
	if err != nil {
		return "", err
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	return u.String(), nil
}

func generatedCLIPath(operationID string) (string, bool) {
	for _, spec := range apigencli.APIGeneratedCommandSpecs {
		if spec.OperationID == operationID {
			return spec.Path, true
		}
	}
	return "", false
}
