package cli

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/flidai/leapview/internal/platform/cliapi"
	"github.com/spf13/cobra"
)

type dataRevisionOptions struct {
	project     string
	connection  string
	environment string
	pagination  cliapi.PaginationOptions
}

func dataRevisionsCommand(ctx context.Context, dependencies Dependencies, opts *options) *cobra.Command {
	parent := &cobra.Command{Use: "revisions", Short: "Inspect managed data revisions"}
	listOptions := dataRevisionOptions{}
	list := &cobra.Command{
		Use:   "list",
		Short: "List staged managed data revisions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := listOptions.pagination.Validate(cmd); err != nil {
				return err
			}
			if err := validateDataRevisionOptions(listOptions, false); err != nil {
				return err
			}
			if dependencies.Client == nil {
				return fmt.Errorf("Managed Data CLI API client is required")
			}
			credentials, err := dependencies.Client.Resolve(ctx, opts.remote.Credentials())
			if err != nil {
				return err
			}
			if _, err := dependencies.Client.Environment(ctx, credentials, listOptions.environment); err != nil {
				return err
			}
			return runDataRevisionsList(ctx, listOptions, newManagedDataCLIClient(dependencies.HTTPClient, credentials.Target, credentials.Token), cmd.OutOrStdout())
		},
	}
	addDataRevisionFlags(list, opts, &listOptions, false)

	currentOptions := dataRevisionOptions{}
	current := &cobra.Command{
		Use:   "current",
		Short: "Print the active managed data revision",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateDataRevisionOptions(currentOptions, true); err != nil {
				return err
			}
			if dependencies.Client == nil {
				return fmt.Errorf("Managed Data CLI API client is required")
			}
			credentials, err := dependencies.Client.Resolve(ctx, opts.remote.Credentials())
			if err != nil {
				return err
			}
			if _, err := dependencies.Client.Environment(ctx, credentials, currentOptions.environment); err != nil {
				return err
			}
			return runDataRevisionCurrent(ctx, currentOptions, newManagedDataCLIClient(dependencies.HTTPClient, credentials.Target, credentials.Token), cmd.OutOrStdout())
		},
	}
	addDataRevisionFlags(current, opts, &currentOptions, true)
	parent.AddCommand(list, current)
	return parent
}

func addDataRevisionFlags(command *cobra.Command, opts *options, values *dataRevisionOptions, current bool) {
	command.Flags().StringVar(&values.project, "project", "", "server project id")
	command.Flags().StringVar(&values.connection, "connection", "", "project-global managed connection")
	command.Flags().StringVar(&values.environment, "environment", "", "assert the target instance environment")
	if !current {
		values.pagination.AddFlags(command)
	}
	opts.remote.AddFlags(command)
}

func validateDataRevisionOptions(values dataRevisionOptions, current bool) error {
	if strings.TrimSpace(values.project) == "" {
		return fmt.Errorf("project is required")
	}
	if strings.TrimSpace(values.connection) == "" {
		return fmt.Errorf("connection is required")
	}
	return nil
}

func runDataRevisionsList(ctx context.Context, values dataRevisionOptions, client *managedDataCLIClient, out io.Writer) error {
	query := url.Values{}
	if values.pagination.Limit > 0 {
		query.Set("limit", strconv.Itoa(values.pagination.Limit))
	}
	if values.pagination.PageToken != "" {
		query.Set("pageToken", values.pagination.PageToken)
	}
	response, err := client.listRevisions(ctx, values.project, values.connection, query)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "REVISION\tSTATUS\tFILES\tBYTES\tCREATED"); err != nil {
		return err
	}
	for _, revision := range response.Items {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%s\n", revision.Id, strings.ToUpper(string(revision.Status)), revision.FileCount, revision.Size, revision.CreatedAt); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func runDataRevisionCurrent(ctx context.Context, values dataRevisionOptions, client *managedDataCLIClient, out io.Writer) error {
	response, err := client.currentRevision(ctx, values.project, values.connection, "")
	if err != nil {
		return err
	}
	if response.Revision == nil {
		_, err = fmt.Fprintln(out, "none")
		return err
	}
	_, err = fmt.Fprintln(out, response.Revision.Id)
	return err
}
