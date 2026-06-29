package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Yacobolo/libredash/internal/configschema"
	workspacecompiler "github.com/Yacobolo/libredash/internal/workspace/compiler"
	"github.com/spf13/cobra"
)

func validateCommand(ctx context.Context, opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [project]",
		Short: "Validate a configuration-as-code project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("validate accepts at most one positional project")
			}
			if len(args) == 1 {
				if cmd.Flags().Changed("catalog") {
					return fmt.Errorf("choose either --catalog or positional project, not both")
				}
				opts.catalog = args[0]
			}
			return runValidate(ctx, opts, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&opts.catalog, "catalog", filepath.Join("dashboards", "libredash.yaml"), "project path")
	cmd.Flags().BoolVar(&opts.jsonOutput, "json", false, "emit JSON diagnostics")
	return cmd
}

func planCommand(ctx context.Context, opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan [project]",
		Short: "Emit a deterministic configuration-as-code plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("plan accepts at most one positional project")
			}
			if len(args) == 1 {
				if cmd.Flags().Changed("catalog") {
					return fmt.Errorf("choose either --catalog or positional project, not both")
				}
				opts.catalog = args[0]
			}
			return runPlan(ctx, opts, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&opts.catalog, "catalog", filepath.Join("dashboards", "libredash.yaml"), "project path")
	cmd.Flags().BoolVar(&opts.jsonOutput, "json", false, "emit JSON plan")
	return cmd
}

func schemaCommand(opts *rootOptions) *cobra.Command {
	parent := &cobra.Command{
		Use:   "schema",
		Short: "Inspect LibreDash YAML schemas",
	}
	export := &cobra.Command{
		Use:   "export",
		Short: "Export generated schema artifacts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSchemaExport(opts)
		},
	}
	export.Flags().StringVar(&opts.schemaFormat, "format", "json-schema", "schema output format")
	export.Flags().StringVar(&opts.schemaOut, "out", filepath.Join("schemas", "json"), "output directory")
	parent.AddCommand(export)
	return parent
}

type validateResponse struct {
	OK          bool                      `json:"ok"`
	Diagnostics []configschema.Diagnostic `json:"diagnostics"`
}

func runValidate(ctx context.Context, opts *rootOptions, out io.Writer) error {
	diagnostics := validateCatalog(ctx, opts.catalog)
	response := validateResponse{OK: len(diagnostics) == 0, Diagnostics: diagnostics}
	if opts.jsonOutput {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(response); err != nil {
			return err
		}
		if response.OK {
			return nil
		}
		return fmt.Errorf("validation failed")
	}
	if response.OK {
		fmt.Fprintf(out, "ok %s\n", opts.catalog)
		return nil
	}
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(out, diagnostic.String())
	}
	return fmt.Errorf("validation failed")
}

func runPlan(ctx context.Context, opts *rootOptions, out io.Writer) error {
	plan, err := workspacecompiler.PlanProject(opts.catalog)
	if err != nil {
		return err
	}
	if opts.jsonOutput {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(plan)
	}
	fmt.Fprintf(out, "project %s\n", plan.Project)
	for _, workspace := range plan.Workspaces {
		fmt.Fprintf(out, "workspace %s\n", workspace.ID)
		fmt.Fprintf(out, "  connections %s\n", strings.Join(workspace.Connections, ","))
		fmt.Fprintf(out, "  sources %s\n", strings.Join(workspace.Sources, ","))
		fmt.Fprintf(out, "  model_tables %s\n", strings.Join(workspace.ModelTables, ","))
		fmt.Fprintf(out, "  semantic_models %s\n", strings.Join(workspace.SemanticModels, ","))
		fmt.Fprintf(out, "  dashboards %s\n", strings.Join(workspace.Dashboards, ","))
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func validateCatalog(ctx context.Context, catalogPath string) []configschema.Diagnostic {
	if _, err := workspacecompiler.CompileProject(catalogPath, workspacecompiler.Options{}); err != nil {
		return configschema.Diagnostics(err)
	}
	if err := ctx.Err(); err != nil {
		return configschema.Diagnostics(err)
	}
	return nil
}

func runSchemaExport(opts *rootOptions) error {
	if opts.schemaFormat != "json-schema" {
		return fmt.Errorf("unsupported schema format %q", opts.schemaFormat)
	}
	files, err := configschema.JSONSchemaFiles()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(opts.schemaOut, 0o755); err != nil {
		return err
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(opts.schemaOut, name), content, 0o644); err != nil {
			return err
		}
	}
	return nil
}
