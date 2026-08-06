// Package cli owns command-line adapters for the Workspace capability.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/flidai/leapview/internal/platform/cliapi"
	workspacegen "github.com/flidai/leapview/internal/workspace/api/gen"
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
			if err := options.pagination.Validate(command); err != nil {
				return err
			}
			api, err := workspaceClient(ctx, client, options.remote.Credentials())
			if err != nil {
				return err
			}
			response, err := api.ListWorkspaces(ctx, workspacegen.GenListWorkspacesClientRequest{
				Params: workspacegen.GenListWorkspacesClientParams{
					Limit:     options.pagination.LimitPtr(),
					PageToken: optionalString(options.pagination.PageToken),
				},
			})
			if err != nil {
				return err
			}
			return json.NewEncoder(command.OutOrStdout()).Encode(response.Body)
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
			if err := options.pagination.Validate(command); err != nil {
				return err
			}
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
	api, err := workspaceClient(ctx, client, options.remote.Credentials())
	if err != nil {
		return err
	}
	workspaceFilters := splitValues(options.workspaces)
	typeValues := splitValues(options.types)
	typeFilters := make([]workspacegen.GenSchemaSearchResultType, len(typeValues))
	for index, value := range typeValues {
		typeFilters[index] = workspacegen.GenSchemaSearchResultType(value)
	}
	response, err := api.Search(ctx, workspacegen.GenSearchClientRequest{
		Params: workspacegen.GenSearchClientParams{
			Q:         &queryText,
			Workspace: optionalSlice(workspaceFilters),
			Type:      optionalSlice(typeFilters),
			Limit:     options.pagination.LimitPtr(),
			PageToken: optionalString(options.pagination.PageToken),
		},
	})
	if err != nil {
		return err
	}
	if options.jsonOutput {
		return json.NewEncoder(out).Encode(response.Body)
	}
	return renderSearchResults(out, response.Body)
}

func workspaceClient(ctx context.Context, client cliapi.Client, credentials cliapi.Credentials) (*workspacegen.GenClient, error) {
	if client == nil {
		return nil, fmt.Errorf("workspace CLI API client is required")
	}
	transport, err := client.Transport(ctx, credentials)
	if err != nil {
		return nil, err
	}
	return workspacegen.NewGenClient(transport), nil
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

func renderSearchResults(out io.Writer, response workspacegen.GenSchemaSearchResponse) error {
	writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "WORKSPACE\tTYPE\tNAME\tDESCRIPTION\tID")
	for _, item := range response.Items {
		description := ""
		if item.Description != nil {
			description = *item.Description
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", item.Reference.WorkspaceId, item.Reference.Type, item.Name, description, item.Reference.Id)
	}
	if response.Page.NextCursor != nil && *response.Page.NextCursor != "" {
		fmt.Fprintf(writer, "PAGE\tNEXT\t%s\t\t\n", *response.Page.NextCursor)
	}
	return writer.Flush()
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func optionalSlice[T any](values []T) *[]T {
	if len(values) == 0 {
		return nil
	}
	return &values
}
