package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

func workspacesCommand(ctx context.Context, opts *rootOptions) *cobra.Command {
	parent := &cobra.Command{Use: "workspaces", Short: "Inspect workspaces"}
	list := &cobra.Command{
		Use:   "list",
		Short: "List workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRawAPI(ctx, opts, "listWorkspaces", nil, nil)
		},
	}
	addTargetTokenFlags(list, opts)
	parent.AddCommand(list)
	return parent
}

func dashboardsCommand(ctx context.Context, opts *rootOptions) *cobra.Command {
	parent := &cobra.Command{Use: "dashboards", Short: "Inspect dashboards"}
	list := &cobra.Command{
		Use:   "list",
		Short: "List dashboards",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRawAPI(ctx, opts, "listDashboards", map[string]string{"workspace": opts.workspaceID}, nil)
		},
	}
	addTargetTokenFlags(list, opts)

	describe := &cobra.Command{
		Use:   "describe <dashboard>",
		Short: "Describe a dashboard",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRawAPI(ctx, opts, "getDashboard", map[string]string{"workspace": opts.workspaceID, "dashboard": args[0]}, nil)
		},
	}
	addTargetTokenFlags(describe, opts)

	queryPage := &cobra.Command{
		Use:   "query-page <dashboard> <page>",
		Short: "Query a dashboard page",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := filtersBody(opts.filtersJSON)
			if err != nil {
				return err
			}
			return runRawAPI(ctx, opts, "queryDashboardPage", map[string]string{"workspace": opts.workspaceID, "dashboard": args[0], "page": args[1]}, body)
		},
	}
	addTargetTokenFlags(queryPage, opts)
	queryPage.Flags().StringVar(&opts.filtersJSON, "filters-json", "", "dashboard filters JSON")

	queryTable := &cobra.Command{
		Use:   "query-table <dashboard> <table>",
		Short: "Query a dashboard table",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := tableQueryBody(opts.pageID, opts.count, opts.filtersJSON)
			if err != nil {
				return err
			}
			return runRawAPI(ctx, opts, "queryDashboardTable", map[string]string{"workspace": opts.workspaceID, "dashboard": args[0], "table": args[1]}, body)
		},
	}
	addTargetTokenFlags(queryTable, opts)
	queryTable.Flags().StringVar(&opts.pageID, "page", "", "dashboard page id")
	queryTable.Flags().IntVar(&opts.count, "count", 0, "row count")
	queryTable.Flags().StringVar(&opts.filtersJSON, "filters-json", "", "dashboard filters JSON")

	parent.AddCommand(list, describe, queryPage, queryTable)
	return parent
}

func semanticModelsCommand(ctx context.Context, opts *rootOptions) *cobra.Command {
	parent := &cobra.Command{Use: "semantic-models", Short: "Inspect semantic models"}
	list := &cobra.Command{
		Use:   "list",
		Short: "List semantic models",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRawAPI(ctx, opts, "listSemanticModels", map[string]string{"workspace": opts.workspaceID}, nil)
		},
	}
	addTargetTokenFlags(list, opts)

	describe := &cobra.Command{
		Use:   "describe <model>",
		Short: "Describe a semantic model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRawAPI(ctx, opts, "getSemanticModel", map[string]string{"workspace": opts.workspaceID, "model": args[0]}, nil)
		},
	}
	addTargetTokenFlags(describe, opts)

	parent.AddCommand(list, describe)
	return parent
}

func addTargetTokenFlags(cmd *cobra.Command, opts *rootOptions) {
	cmd.Flags().StringVar(&opts.target, "target", "", "LibreDash server URL")
	cmd.Flags().StringVar(&opts.token, "token", "", "API token")
}

func runRawAPI(ctx context.Context, opts *rootOptions, operationID string, pathParams map[string]string, body map[string]any) error {
	target, token, err := clientTargetAndToken(opts)
	if err != nil {
		return err
	}
	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	}
	endpoint, err := apiOperationURL(target, operationID, pathParams, url.Values{})
	if err != nil {
		return err
	}
	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}
	var out any
	var requestBody *bytes.Reader
	if body != nil {
		requestBody = reader
	}
	if err := doJSON(ctx, method, endpoint, token, requestBody, &out); err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	return encoder.Encode(out)
}

func filtersBody(raw string) (map[string]any, error) {
	if raw == "" {
		return nil, nil
	}
	filters, err := decodeObjectJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("filters-json: %w", err)
	}
	return map[string]any{"filters": filters}, nil
}

func tableQueryBody(pageID string, count int, rawFilters string) (map[string]any, error) {
	body := map[string]any{}
	if pageID != "" {
		body["pageId"] = pageID
	}
	if count > 0 {
		body["count"] = count
	}
	if rawFilters != "" {
		filters, err := decodeObjectJSON(rawFilters)
		if err != nil {
			return nil, fmt.Errorf("filters-json: %w", err)
		}
		body["filters"] = filters
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
