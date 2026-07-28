// Package cli owns command-line adapters for the Workspace capability.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/tabwriter"

	"github.com/Yacobolo/leapview/internal/platform/cliapi"
	"github.com/spf13/cobra"
)

type listOptions struct {
	remote     cliapi.RemoteOptions
	pagination cliapi.PaginationOptions
}

// WorkspacesCommand constructs the workspace inspection command.
func WorkspacesCommand(ctx context.Context, client cliapi.Client) *cobra.Command {
	options := &listOptions{}
	parent := &cobra.Command{Use: "workspaces", Short: "Inspect workspaces"}
	list := &cobra.Command{
		Use:   "list",
		Short: "List workspaces",
		RunE: func(command *cobra.Command, _ []string) error {
			return runRaw(ctx, client, options.remote.Credentials(), cliapi.Request{
				Method:      http.MethodGet,
				OperationID: "listWorkspaces",
				Query:       options.pagination.Query(),
			}, command.OutOrStdout())
		},
	}
	options.remote.AddFlags(list)
	options.pagination.AddFlags(list)
	parent.AddCommand(list)
	return parent
}

type searchOptions struct {
	remote     cliapi.RemoteOptions
	pagination cliapi.PaginationOptions
	workspaces []string
	types      []string
	jsonOutput bool
}

// SearchCommand constructs the workspace-owned product search command.
func SearchCommand(ctx context.Context, client cliapi.Client) *cobra.Command {
	options := &searchOptions{}
	command := &cobra.Command{
		Use:   "search <query>",
		Short: "Search accessible product objects",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return runSearch(ctx, client, options, args[0], command.OutOrStdout())
		},
	}
	options.remote.AddFlags(command)
	command.Flags().StringArrayVar(&options.workspaces, "workspace", nil, "workspace filter; repeatable")
	options.pagination.AddFlags(command)
	command.Flags().StringArrayVar(&options.types, "type", nil, "result type filter; repeatable or comma-separated")
	command.Flags().BoolVar(&options.jsonOutput, "json", false, "print JSON response")
	return command
}

func runSearch(ctx context.Context, client cliapi.Client, options *searchOptions, queryText string, out io.Writer) error {
	query := options.pagination.Query()
	query.Set("q", queryText)
	for _, workspaceID := range splitValues(options.workspaces) {
		query.Add("workspace", workspaceID)
	}
	for _, typ := range splitValues(options.types) {
		query.Add("type", typ)
	}
	var response searchResponse
	if err := doJSON(ctx, client, options.remote.Credentials(), cliapi.Request{
		Method: http.MethodGet, OperationID: "search", Query: query,
	}, &response); err != nil {
		return err
	}
	if options.jsonOutput {
		return json.NewEncoder(out).Encode(response)
	}
	return renderSearchResults(out, response)
}

func runRaw(ctx context.Context, client cliapi.Client, credentials cliapi.Credentials, request cliapi.Request, out io.Writer) error {
	var response any
	if err := doJSON(ctx, client, credentials, request, &response); err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(response)
}

func doJSON(ctx context.Context, client cliapi.Client, credentials cliapi.Credentials, request cliapi.Request, out any) error {
	if client == nil {
		return fmt.Errorf("workspace CLI API client is required")
	}
	return client.DoJSON(ctx, credentials, request, out)
}

func splitValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}

func renderSearchResults(out io.Writer, response searchResponse) error {
	writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "WORKSPACE\tTYPE\tNAME\tDESCRIPTION\tID")
	for _, item := range response.Items {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", item.Reference.WorkspaceID, item.Reference.Type, item.Name, item.Description, item.Reference.ID)
	}
	if response.Page.NextCursor != "" {
		fmt.Fprintf(writer, "PAGE\tNEXT\t%s\t\t\n", response.Page.NextCursor)
	}
	return writer.Flush()
}

type searchResponse struct {
	Items []searchResult `json:"items"`
	Page  struct {
		NextCursor string `json:"nextCursor"`
	} `json:"page"`
}

type searchResult struct {
	Reference   searchReference `json:"reference"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Href        string          `json:"href"`
}

type searchReference struct {
	WorkspaceID string `json:"workspaceId"`
	Type        string `json:"type"`
	ID          string `json:"id"`
}
