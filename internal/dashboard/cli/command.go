// Package cli owns command-line adapters for the Dashboard capability.
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
	remote          cliapi.RemoteOptions
	pagination      cliapi.PaginationOptions
	workspaceID     string
	count           int
	filterStateJSON string
}

// Command constructs the dashboard inspection and query command.
func Command(ctx context.Context, client cliapi.Client, defaultWorkspaceID string) *cobra.Command {
	values := &options{workspaceID: defaultWorkspaceID}
	parent := &cobra.Command{Use: "dashboards", Short: "Inspect dashboards"}
	parent.PersistentFlags().StringVar(&values.workspaceID, "workspace", values.workspaceID, "workspace id")

	list := requestCommand(ctx, client, values, commandSpec{
		use: "list", short: "List dashboards", operationID: "listDashboards",
		pathParams: func(_ []string) map[string]string {
			return map[string]string{"workspace": values.workspaceID}
		},
		query: values.pagination.Query,
	})
	values.pagination.AddFlags(list)

	describe := requestCommand(ctx, client, values, commandSpec{
		use: "describe <dashboard>", short: "Describe a dashboard", operationID: "getDashboard", exactArgs: 1,
		pathParams: func(args []string) map[string]string {
			return map[string]string{"workspace": values.workspaceID, "dashboard": args[0]}
		},
	})
	page := requestCommand(ctx, client, values, commandSpec{
		use: "page <dashboard> <page>", short: "Describe a dashboard page", operationID: "getDashboardPage", exactArgs: 2,
		pathParams: func(args []string) map[string]string {
			return map[string]string{"workspace": values.workspaceID, "dashboard": args[0], "page": args[1]}
		},
	})
	visual := requestCommand(ctx, client, values, commandSpec{
		use: "visual <dashboard> <page> <visual>", short: "Describe a dashboard visual", operationID: "getDashboardVisual", exactArgs: 3,
		pathParams: func(args []string) map[string]string {
			return dashboardPath(values.workspaceID, args)
		},
	})
	filter := requestCommand(ctx, client, values, commandSpec{
		use: "filter <dashboard> <page> <filter>", short: "Describe a compiled dashboard filter binding", operationID: "getDashboardFilter", exactArgs: 3,
		pathParams: func(args []string) map[string]string {
			return map[string]string{"workspace": values.workspaceID, "dashboard": args[0], "page": args[1], "filter": args[2]}
		},
	})
	visualData := requestCommand(ctx, client, values, commandSpec{
		use: "visual-data <dashboard> <page> <visual>", short: "Query dashboard visual data", operationID: "queryDashboardVisualData", exactArgs: 3,
		pathParams: func(args []string) map[string]string {
			return dashboardPath(values.workspaceID, args)
		},
		body: func() (any, error) {
			return visualQueryBody(values.count, values.filterStateJSON)
		},
	})
	visualData.Flags().StringVar(&values.filterStateJSON, "filter-state-json", "", "versioned dashboard filter state JSON")
	visualData.Flags().IntVar(&values.count, "count", 0, "row count for table, matrix, or pivot visuals")

	queryPage := requestCommand(ctx, client, values, commandSpec{
		use: "query-page <dashboard> <page>", short: "Query a dashboard page", operationID: "queryDashboardPage", exactArgs: 2,
		pathParams: func(args []string) map[string]string {
			return map[string]string{"workspace": values.workspaceID, "dashboard": args[0], "page": args[1]}
		},
		body: func() (any, error) {
			return filterStateBody(values.filterStateJSON)
		},
	})
	queryPage.Flags().StringVar(&values.filterStateJSON, "filter-state-json", "", "versioned dashboard filter state JSON")

	filterOptions := requestCommand(ctx, client, values, commandSpec{
		use: "filter-options <dashboard> <page> <filter>", short: "List dashboard filter options", operationID: "listDashboardFilterValues", exactArgs: 3,
		pathParams: func(args []string) map[string]string {
			return map[string]string{"workspace": values.workspaceID, "dashboard": args[0], "page": args[1], "filter": args[2]}
		},
		query: values.pagination.Query,
		body: func() (any, error) {
			return filterStateBody(values.filterStateJSON)
		},
	})
	values.pagination.AddFlags(filterOptions)
	filterOptions.Flags().StringVar(&values.filterStateJSON, "filter-state-json", "", "versioned dashboard filter state JSON")

	parent.AddCommand(list, describe, page, visual, filter, visualData, queryPage, filterOptions)
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

func runRequest(ctx context.Context, client cliapi.Client, credentials cliapi.Credentials, request cliapi.Request, out io.Writer) error {
	if client == nil {
		return fmt.Errorf("dashboard CLI API client is required")
	}
	var response any
	if err := client.DoJSON(ctx, credentials, request, &response); err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(response)
}

func dashboardPath(workspaceID string, args []string) map[string]string {
	return map[string]string{"workspace": workspaceID, "dashboard": args[0], "page": args[1], "visual": args[2]}
}

func filterStateBody(raw string) (map[string]any, error) {
	if raw == "" {
		return nil, nil
	}
	filterState, err := decodeObjectJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("filter-state-json: %w", err)
	}
	return map[string]any{"filterState": filterState}, nil
}

func visualQueryBody(count int, rawFilterState string) (map[string]any, error) {
	body := map[string]any{}
	if count > 0 {
		body["limit"] = count
	}
	if rawFilterState != "" {
		filterState, err := decodeObjectJSON(rawFilterState)
		if err != nil {
			return nil, fmt.Errorf("filter-state-json: %w", err)
		}
		body["filterState"] = filterState
	}
	if len(body) == 0 {
		return nil, nil
	}
	return body, nil
}

func decodeObjectJSON(raw string) (map[string]any, error) {
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	if out == nil {
		return nil, fmt.Errorf("must be a JSON object")
	}
	return out, nil
}
