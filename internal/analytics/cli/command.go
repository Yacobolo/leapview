// Package cli owns command-line adapters for the Analytics capability.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/Yacobolo/leapview/internal/platform/cliapi"
	"github.com/spf13/cobra"
)

type options struct {
	remote      cliapi.RemoteOptions
	pagination  cliapi.PaginationOptions
	workspaceID string
	bodyJSON    string
}

// Command constructs the semantic-model inspection and query command.
func Command(ctx context.Context, client cliapi.Client, defaultWorkspaceID string) *cobra.Command {
	values := &options{workspaceID: defaultWorkspaceID}
	parent := &cobra.Command{Use: "semantic-models", Short: "Inspect semantic models"}
	parent.PersistentFlags().StringVar(&values.workspaceID, "workspace", values.workspaceID, "workspace id")

	list := requestCommand(ctx, client, values, commandSpec{
		use: "list", short: "List semantic models", operationID: "listSemanticModels",
		pathParams: func(_ []string) map[string]string {
			return map[string]string{"workspace": values.workspaceID}
		},
		query: values.pagination.Query,
	})
	values.pagination.AddFlags(list)
	describe := requestCommand(ctx, client, values, commandSpec{
		use: "describe <model>", short: "Describe a semantic model", operationID: "getSemanticModel", exactArgs: 1,
		pathParams: func(args []string) map[string]string {
			return modelPath(values.workspaceID, args[0])
		},
	})
	datasets := requestCommand(ctx, client, values, commandSpec{
		use: "datasets <model>", short: "List semantic model datasets", operationID: "listSemanticDatasets", exactArgs: 1,
		pathParams: func(args []string) map[string]string {
			return modelPath(values.workspaceID, args[0])
		},
		query: values.pagination.Query,
	})
	values.pagination.AddFlags(datasets)
	dataset := requestCommand(ctx, client, values, commandSpec{
		use: "dataset <model> <dataset>", short: "Describe a semantic model dataset", operationID: "getSemanticDataset", exactArgs: 2,
		pathParams: func(args []string) map[string]string {
			return datasetPath(values.workspaceID, args)
		},
	})
	fields := requestCommand(ctx, client, values, commandSpec{
		use: "fields <model> <dataset>", short: "List semantic model dataset fields", operationID: "listSemanticFields", exactArgs: 2,
		pathParams: func(args []string) map[string]string {
			return datasetPath(values.workspaceID, args)
		},
		query: values.pagination.Query,
	})
	values.pagination.AddFlags(fields)

	query := bodyCommand(ctx, client, values, "query <model>", "Query governed semantic data", "querySemanticModel", false)
	preview := bodyCommand(ctx, client, values, "preview <model> <dataset>", "Preview semantic model dataset rows", "previewSemanticDataset", true)
	explainQuery := bodyCommand(ctx, client, values, "explain-query <model>", "Explain a governed semantic query", "explainSemanticModelQuery", false)
	explainPreview := bodyCommand(ctx, client, values, "explain-preview <model> <dataset>", "Explain a semantic model dataset row preview", "explainSemanticPreview", true)

	parent.AddCommand(list, describe, datasets, dataset, fields, query, preview, explainQuery, explainPreview)
	return parent
}

type commandSpec struct {
	use         string
	short       string
	operationID string
	exactArgs   int
	pathParams  func([]string) map[string]string
	query       func() url.Values
	body        func() (any, error)
}

func requestCommand(ctx context.Context, client cliapi.Client, values *options, spec commandSpec) *cobra.Command {
	command := &cobra.Command{
		Use:   spec.use,
		Short: spec.short,
		RunE: func(command *cobra.Command, args []string) error {
			request := cliapi.Request{Method: http.MethodGet, OperationID: spec.operationID}
			if spec.pathParams != nil {
				request.PathParams = spec.pathParams(args)
			}
			if spec.query != nil {
				request.Query = spec.query()
			}
			if spec.body != nil {
				body, err := spec.body()
				if err != nil {
					return err
				}
				if body == nil {
					body = map[string]any{}
				}
				request.Method = http.MethodPost
				request.Body = body
			}
			return runRequest(ctx, client, values.remote.Credentials(), request, command.OutOrStdout())
		},
	}
	if spec.exactArgs > 0 {
		command.Args = cobra.ExactArgs(spec.exactArgs)
	}
	values.remote.AddFlags(command)
	return command
}

func bodyCommand(ctx context.Context, client cliapi.Client, values *options, use, short, operationID string, hasDataset bool) *cobra.Command {
	exactArgs := 1
	if hasDataset {
		exactArgs = 2
	}
	command := requestCommand(ctx, client, values, commandSpec{
		use: use, short: short, operationID: operationID, exactArgs: exactArgs,
		pathParams: func(args []string) map[string]string {
			if hasDataset {
				return datasetPath(values.workspaceID, args)
			}
			return modelPath(values.workspaceID, args[0])
		},
		body: func() (any, error) {
			return bodyJSONMap(values.bodyJSON)
		},
	})
	command.Flags().StringVar(&values.bodyJSON, "body-json", "", "request JSON body")
	return command
}

func runRequest(ctx context.Context, client cliapi.Client, credentials cliapi.Credentials, request cliapi.Request, out io.Writer) error {
	if client == nil {
		return fmt.Errorf("analytics CLI API client is required")
	}
	var response any
	if err := client.DoJSON(ctx, credentials, request, &response); err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(response)
}

func modelPath(workspaceID, modelID string) map[string]string {
	return map[string]string{"workspace": workspaceID, "model": modelID}
}

func datasetPath(workspaceID string, args []string) map[string]string {
	return map[string]string{"workspace": workspaceID, "model": args[0], "dataset": args[1]}
}

func bodyJSONMap(raw string) (map[string]any, error) {
	if raw == "" {
		return nil, nil
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return nil, fmt.Errorf("body-json: %w", err)
	}
	if body == nil {
		return nil, fmt.Errorf("body-json: must be a JSON object")
	}
	return body, nil
}
