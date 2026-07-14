package cli

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/Yacobolo/libredash/internal/api"
	servingstate "github.com/Yacobolo/libredash/internal/servingstate"
	servingstatefs "github.com/Yacobolo/libredash/internal/servingstate/filesystem"
	"github.com/Yacobolo/libredash/internal/workspace"
	workspacecompiler "github.com/Yacobolo/libredash/internal/workspace/compiler"
	"github.com/spf13/cobra"
)

func publishCommand(ctx context.Context, opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish a configuration-as-code project",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPublish(ctx, opts)
		},
	}
	cmd.Flags().StringVar(&opts.target, "target", "", "LibreDash server URL")
	cmd.Flags().StringVar(&opts.token, "token", "", "API token")
	cmd.Flags().StringVar(&opts.catalog, "project", filepath.Join("dashboards", "libredash.yaml"), "project path")
	cmd.Flags().StringVar(&opts.environment, "environment", "dev", "publish environment")
	cmd.Flags().BoolVar(&opts.autoApprove, "auto-approve", false, "approve and activate the publish without prompting")
	return cmd
}

func publishesCommand(ctx context.Context, opts *rootOptions) *cobra.Command {
	parent := &cobra.Command{Use: "publishes", Short: "Inspect publishes"}
	list := &cobra.Command{
		Use:   "list",
		Short: "List publishes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPublishesList(ctx, opts)
		},
	}
	list.Flags().StringVar(&opts.target, "target", "", "LibreDash server URL")
	list.Flags().StringVar(&opts.token, "token", "", "API token")
	list.Flags().StringVar(&opts.environment, "environment", "dev", "publish environment")
	addPaginationFlags(list, opts)
	parent.AddCommand(list)
	return parent
}

func runPublish(ctx context.Context, opts *rootOptions) error {
	target, token, err := clientTargetAndToken(opts)
	if err != nil {
		return err
	}
	project, err := workspacecompiler.LoadProject(opts.catalog)
	if err != nil {
		return err
	}
	workspaceIDs := sortedPublishWorkspaceIDs(project.Workspaces, opts.workspaceID)
	if len(workspaceIDs) == 0 {
		if opts.workspaceID != "" {
			return fmt.Errorf("project %q has no workspace %q", opts.catalog, opts.workspaceID)
		}
		return fmt.Errorf("project %q has no workspaces", opts.catalog)
	}

	results := make([]publishWorkspaceResult, 0, len(workspaceIDs))
	needsApproval := false
	for _, workspaceID := range workspaceIDs {
		result := publishWorkspaceResult{WorkspaceID: workspaceID}
		activeGraph, err := fetchActiveWorkspaceGraphFor(ctx, opts, workspaceID)
		if err != nil {
			result.Err = err
			results = append(results, result)
			continue
		}
		plan, err := workspacecompiler.PlanProjectAgainstGraph(opts.catalog, workspaceID, activeGraph)
		if err != nil {
			result.Err = err
			results = append(results, result)
			continue
		}
		workspacePlan := plan.Workspaces[0]
		if len(workspaceIDs) == 1 {
			if err := renderProjectPlan(os.Stdout, plan); err != nil {
				return err
			}
		} else {
			printPublishPlanSummary(workspacePlan)
		}
		if projectPlanWorkspaceUnchanged(workspacePlan) {
			result.Status = "skipped"
			results = append(results, result)
			continue
		}
		result.Status = "pending"
		result.ActiveGraph = activeGraph
		result.ManagedConnections = managedConnectionsForWorkspace(project, project.Workspaces[workspaceID])
		needsApproval = true
		results = append(results, result)
	}
	if needsApproval {
		if err := confirmPublish(opts, os.Stdin, os.Stdout); err != nil {
			return err
		}
	}
	managedDataPins, err := fetchManagedDataPins(ctx, target, token, project.Name, cliEnvironment(opts), results)
	if err != nil {
		return err
	}

	var failures []string
	for index := range results {
		result := &results[index]
		if result.Err != nil {
			result.Status = "failed"
			failures = append(failures, fmt.Sprintf("%s: %v", result.WorkspaceID, result.Err))
			fmt.Printf("failed %s: %v\n", result.WorkspaceID, result.Err)
			continue
		}
		if result.Status == "skipped" {
			fmt.Printf("skipped %s unchanged\n", result.WorkspaceID)
			continue
		}
		workspaceProject := project.Workspaces[result.WorkspaceID]
		activated, digest, err := publishWorkspace(ctx, opts, target, token, result.WorkspaceID, workspaceProject, result.ActiveGraph, selectManagedDataPins(managedDataPins, result.ManagedConnections))
		if err != nil {
			result.Status = "failed"
			failures = append(failures, fmt.Sprintf("%s: %v", result.WorkspaceID, err))
			fmt.Printf("failed %s: %v\n", result.WorkspaceID, err)
			continue
		}
		result.Status = "published"
		fmt.Printf("published %s publish=%s environment=%s digest=%s localDigest=%s status=%s\n", result.WorkspaceID, activated.ID, activated.Environment, activated.Digest, digest, activated.Status)
	}
	if len(failures) > 0 {
		return fmt.Errorf("publish failed: %s", strings.Join(failures, "; "))
	}
	return nil
}

type publishWorkspaceResult struct {
	WorkspaceID        string
	Status             string
	ActiveGraph        workspace.AssetGraph
	ManagedConnections []string
	Err                error
}

func managedConnectionsForWorkspace(project workspacecompiler.Project, workspaceProject *workspacecompiler.WorkspaceProject) []string {
	connections := map[string]struct{}{}
	for sourceID := range workspaceProject.AllowedSources {
		source, ok := project.Sources[sourceID]
		if !ok {
			continue
		}
		connection, ok := project.Connections[source.Connection]
		if ok && connection.Kind == "managed" {
			connections[source.Connection] = struct{}{}
		}
	}
	result := make([]string, 0, len(connections))
	for connection := range connections {
		result = append(result, connection)
	}
	sort.Strings(result)
	return result
}

func fetchManagedDataPins(ctx context.Context, target, token, projectID, environment string, results []publishWorkspaceResult) (map[string]string, error) {
	connections := map[string]struct{}{}
	for _, result := range results {
		if result.Err != nil || result.Status != "pending" {
			continue
		}
		for _, connection := range result.ManagedConnections {
			connections[connection] = struct{}{}
		}
	}
	names := make([]string, 0, len(connections))
	for connection := range connections {
		names = append(names, connection)
	}
	sort.Strings(names)
	pins := make(map[string]string, len(names))
	if len(names) == 0 {
		return pins, nil
	}
	client := newManagedDataCLIClient(http.DefaultClient, target, token)
	for _, connection := range names {
		current, err := client.currentRevision(ctx, projectID, connection, environment)
		if err != nil {
			return nil, fmt.Errorf("resolve managed data connection %q current revision: %w", connection, err)
		}
		if current.Environment != environment || current.Revision == nil || current.Revision.Status != "available" || !canonicalManagedRevisionID(current.Revision.Id) {
			return nil, fmt.Errorf("managed data connection %q has no valid current revision in environment %q", connection, environment)
		}
		pins[connection] = current.Revision.Id
	}
	return pins, nil
}

func selectManagedDataPins(all map[string]string, connections []string) map[string]string {
	pins := make(map[string]string, len(connections))
	for _, connection := range connections {
		pins[connection] = all[connection]
	}
	return pins
}

func canonicalManagedRevisionID(value string) bool {
	const prefix = "sha256:"
	if len(value) != len(prefix)+sha256.Size*2 || !strings.HasPrefix(value, prefix) {
		return false
	}
	digest := value[len(prefix):]
	if strings.ToLower(digest) != digest {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}

func sortedPublishWorkspaceIDs(workspaces map[string]*workspacecompiler.WorkspaceProject, filter string) []string {
	if filter != "" {
		if _, ok := workspaces[filter]; !ok {
			return nil
		}
		return []string{filter}
	}
	ids := make([]string, 0, len(workspaces))
	for id := range workspaces {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func projectPlanWorkspaceUnchanged(workspacePlan workspacecompiler.ProjectPlanWorkspace) bool {
	return workspacePlan.Summary.Added == 0 &&
		workspacePlan.Summary.Changed == 0 &&
		workspacePlan.Summary.Removed == 0 &&
		workspacePlan.Summary.DependencyChanges == 0 &&
		len(workspacePlan.Changes) == 0 &&
		len(workspacePlan.DependencyChanges) == 0
}

func printPublishPlanSummary(workspacePlan workspacecompiler.ProjectPlanWorkspace) {
	summary := workspacePlan.Summary
	fmt.Printf("workspace %s changes +%d ~%d -%d dependencies %d\n", workspacePlan.ID, summary.Added, summary.Changed, summary.Removed, summary.DependencyChanges)
}

func publishWorkspace(ctx context.Context, opts *rootOptions, target, token, workspaceID string, workspaceProject *workspacecompiler.WorkspaceProject, activeGraph workspace.AssetGraph, managedDataRevisions map[string]string) (api.PublishResponse, string, error) {
	createBody, _ := json.Marshal(map[string]any{
		"title":       workspaceProject.Title,
		"description": workspaceProject.Description,
		"environment": cliEnvironment(opts),
	})
	var created api.PublishResponse
	workspacePathParams := map[string]string{"workspace": workspaceID}
	createURL, err := apiOperationURL(target, "createPublish", workspacePathParams, environmentQuery(opts, nil))
	if err != nil {
		return api.PublishResponse{}, "", err
	}
	if err := doJSON(ctx, http.MethodPost, createURL, token, bytes.NewReader(createBody), &created); err != nil {
		return api.PublishResponse{}, "", err
	}
	uploadURL, err := apiOperationURL(target, "uploadPublishArtifact", map[string]string{"workspace": workspaceID, "publish": created.ID}, environmentQuery(opts, nil))
	if err != nil {
		return api.PublishResponse{}, "", err
	}
	digest, err := streamProjectArtifact(ctx, uploadURL, token, opts.catalog, servingstatefs.PackProjectOptions{
		WorkspaceID: workspaceID, Environment: servingstate.Environment(cliEnvironment(opts)), ServingStateID: servingstate.ID(created.ID),
		ActiveGraph: activeGraph, ManagedDataRevisions: managedDataRevisions,
	})
	if err != nil {
		return api.PublishResponse{}, "", err
	}
	var validated api.PublishResponse
	validateURL, err := apiOperationURL(target, "validatePublish", map[string]string{"workspace": workspaceID, "publish": created.ID}, environmentQuery(opts, nil))
	if err != nil {
		return api.PublishResponse{}, "", err
	}
	if err := doJSON(ctx, http.MethodPost, validateURL, token, nil, &validated); err != nil {
		return api.PublishResponse{}, "", err
	}
	var activated api.PublishResponse
	activateURL, err := apiOperationURL(target, "activatePublish", map[string]string{"workspace": workspaceID, "publish": created.ID}, environmentQuery(opts, nil))
	if err != nil {
		return api.PublishResponse{}, "", err
	}
	if err := doJSON(ctx, http.MethodPost, activateURL, token, nil, &activated); err != nil {
		return api.PublishResponse{}, "", err
	}
	return activated, digest, nil
}

func streamProjectArtifact(ctx context.Context, uploadURL, token, projectPath string, options servingstatefs.PackProjectOptions) (string, error) {
	reader, writer := io.Pipe()
	type packResult struct {
		digest string
		err    error
	}
	result := make(chan packResult, 1)
	go func() {
		_, digest, err := servingstatefs.PackProject(projectPath, options, writer)
		_ = writer.CloseWithError(err)
		result <- packResult{digest: digest, err: err}
	}()
	uploadErr := doRawAPI(ctx, http.MethodPut, uploadURL, token, "application/gzip", reader, io.Discard)
	_ = reader.CloseWithError(uploadErr)
	packed := <-result
	if packed.err != nil {
		return "", packed.err
	}
	if uploadErr != nil {
		return "", uploadErr
	}
	return packed.digest, nil
}

func runPublishesList(ctx context.Context, opts *rootOptions) error {
	if opts.workspaceID == "" {
		return fmt.Errorf("publishes list requires --workspace")
	}
	target, token, err := clientTargetAndToken(opts)
	if err != nil {
		return err
	}
	listURL, err := apiOperationURL(target, "listPublishes", map[string]string{"workspace": opts.workspaceID}, environmentQuery(opts, paginationQuery(opts)))
	if err != nil {
		return err
	}
	var response apiListResponse[api.PublishResponse]
	if err := doJSON(ctx, http.MethodGet, listURL, token, nil, &response); err != nil {
		return err
	}
	rows := response.Items
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tENVIRONMENT\tSTATUS\tDIGEST\tCREATED\tACTIVATED")
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", row.ID, row.Environment, row.Status, shortDigest(row.Digest), row.CreatedAt, row.ActivatedAt)
	}
	return tw.Flush()
}

func cliEnvironment(opts *rootOptions) string {
	if opts.environment == "" {
		return "dev"
	}
	return opts.environment
}

func confirmPublish(opts *rootOptions, in *os.File, out io.Writer) error {
	if opts.autoApprove {
		return nil
	}
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return fmt.Errorf("publish requires --auto-approve when stdin is not interactive")
	}
	fmt.Fprint(out, "Activate this publish? Type yes to continue: ")
	answer, err := bufio.NewReader(in).ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return fmt.Errorf("publish requires --auto-approve when stdin is not interactive")
		}
		return err
	}
	if strings.TrimSpace(strings.ToLower(answer)) != "yes" {
		return fmt.Errorf("publish activation cancelled")
	}
	return nil
}
