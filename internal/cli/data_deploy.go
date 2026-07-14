package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	apigenapi "github.com/Yacobolo/libredash/internal/api/gen"
	"github.com/Yacobolo/libredash/internal/workspace"
	workspacecompiler "github.com/Yacobolo/libredash/internal/workspace/compiler"
	"github.com/spf13/cobra"
)

type dataDeployRequest struct {
	ProjectPath string
	Connection  string
	Revision    string
	Environment string
	Target      string
	Token       string
	AutoApprove bool
	Out         io.Writer
	HTTPClient  *http.Client
}

func dataDeployCommand(ctx context.Context, opts *rootOptions) *cobra.Command {
	var projectPath string
	var connection string
	var revision string
	var environment string
	var autoApprove bool
	command := &cobra.Command{
		Use:   "deploy",
		Short: "Atomically deploy a staged managed data revision",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			target, token, err := clientTargetAndToken(opts)
			if err != nil {
				return err
			}
			return runDataDeploy(ctx, dataDeployRequest{
				ProjectPath: projectPath, Connection: connection, Revision: revision, Environment: environment,
				Target: target, Token: token, AutoApprove: autoApprove, Out: cmd.OutOrStdout(), HTTPClient: http.DefaultClient,
			})
		},
	}
	command.Flags().StringVar(&projectPath, "project", filepath.Join("dashboards", "libredash.yaml"), "project catalog path")
	command.Flags().StringVar(&connection, "connection", "", "project-global managed connection")
	command.Flags().StringVar(&revision, "revision", "", "staged revision in canonical sha256 form")
	command.Flags().StringVar(&environment, "environment", "dev", "serving environment")
	command.Flags().BoolVar(&autoApprove, "auto-approve", false, "approve the atomic rollout without prompting")
	addTargetTokenFlags(command, opts)
	return command
}

func runDataDeploy(ctx context.Context, request dataDeployRequest) error {
	request.ProjectPath = strings.TrimSpace(request.ProjectPath)
	request.Connection = strings.TrimSpace(request.Connection)
	request.Revision = strings.TrimSpace(request.Revision)
	request.Environment = strings.TrimSpace(request.Environment)
	if ctx == nil || request.ProjectPath == "" || request.Connection == "" || request.Environment == "" {
		return fmt.Errorf("data deploy requires project, connection, revision, and environment")
	}
	if !canonicalManagedRevisionID(request.Revision) {
		return fmt.Errorf("revision must be canonical sha256:<64 lowercase hex>")
	}
	project, err := workspacecompiler.LoadProject(request.ProjectPath)
	if err != nil {
		return fmt.Errorf("load project: %w", err)
	}
	connection, ok := project.Connections[request.Connection]
	if !ok || connection.Kind != "managed" {
		return fmt.Errorf("connection %q is not a managed project connection", request.Connection)
	}
	if _, err := workspacecompiler.CompileProject(request.ProjectPath, workspacecompiler.Options{ServingStateID: "data-deploy-preflight"}); err != nil {
		return fmt.Errorf("compile project: %w", err)
	}

	workspaceIDs := affectedManagedDataWorkspaces(project, request.Connection)
	if len(workspaceIDs) == 0 {
		return fmt.Errorf("managed connection %q is not used by any workspace", request.Connection)
	}
	otherConnections := otherManagedConnections(project, workspaceIDs, request.Connection)
	client := newManagedDataCLIClient(request.HTTPClient, request.Target, request.Token)
	currentPins := make(map[string]string, len(otherConnections)+1)
	currentPins[request.Connection] = request.Revision
	for _, name := range otherConnections {
		current, currentErr := client.currentRevision(ctx, project.Name, name, request.Environment)
		if currentErr != nil {
			return fmt.Errorf("resolve current revision for managed connection %q", name)
		}
		if current.Environment != request.Environment || current.Revision == nil || current.Revision.Status != apigenapi.ManagedDataRevisionStatusAvailable || !canonicalManagedRevisionID(current.Revision.Id) {
			return fmt.Errorf("managed connection %q has no valid current revision in environment %q", name, request.Environment)
		}
		currentPins[name] = current.Revision.Id
	}

	cliOpts := &rootOptions{target: request.Target, token: request.Token, catalog: request.ProjectPath, environment: request.Environment, autoApprove: request.AutoApprove}
	type candidate struct {
		workspaceID string
		connections []string
		activeGraph workspace.AssetGraph
	}
	candidates := make([]candidate, 0, len(workspaceIDs))
	for _, workspaceID := range workspaceIDs {
		graph, graphErr := fetchActiveWorkspaceGraphFor(ctx, cliOpts, workspaceID)
		if graphErr != nil {
			return fmt.Errorf("read active graph for workspace %q", workspaceID)
		}
		plan, planErr := workspacecompiler.PlanProjectAgainstGraph(request.ProjectPath, workspaceID, graph)
		if planErr != nil {
			return fmt.Errorf("plan workspace %q: %w", workspaceID, planErr)
		}
		printPublishPlanSummary(plan.Workspaces[0])
		candidates = append(candidates, candidate{
			workspaceID: workspaceID,
			connections: managedConnectionsForWorkspace(project, project.Workspaces[workspaceID]),
			activeGraph: graph,
		})
	}
	if err := confirmPublish(cliOpts, os.Stdin, outputOrDiscard(request.Out)); err != nil {
		return err
	}

	targets := make([]apigenapi.ManagedDataRolloutTargetRequest, 0, len(candidates))
	for _, item := range candidates {
		pins := selectManagedDataPins(currentPins, item.connections)
		prepared, localDigest, prepareErr := prepareWorkspacePublish(ctx, cliOpts, request.Target, request.Token, item.workspaceID, project.Workspaces[item.workspaceID], item.activeGraph, pins)
		if prepareErr != nil {
			return fmt.Errorf("prepare workspace %q failed", item.workspaceID)
		}
		if prepared.Digest != localDigest || !canonicalArtifactDigest(prepared.Digest) {
			return fmt.Errorf("prepare workspace %q returned an invalid artifact digest", item.workspaceID)
		}
		targets = append(targets, apigenapi.ManagedDataRolloutTargetRequest{Workspace: item.workspaceID, ServingStateId: prepared.ID})
	}

	createRequest := apigenapi.ManagedDataRolloutCreateRequest{Environment: request.Environment, RevisionId: request.Revision, Targets: targets}
	createKeyValues := []string{project.Name, request.Connection, request.Revision, request.Environment}
	createKeyValues = append(createKeyValues, rolloutTargetValues(targets)...)
	createKey := dataDeployIdempotencyKey("create", createKeyValues...)
	created, err := client.createRollout(ctx, project.Name, request.Connection, createKey, createRequest)
	if err != nil {
		return fmt.Errorf("create managed data rollout failed")
	}
	if err := validateRolloutResponse(created, "", request.Revision, request.Environment, apigenapi.ManagedDataRolloutStatusDraft, apigenapi.ManagedDataRolloutTargetStatusPending, targets); err != nil {
		return err
	}
	activateKey := dataDeployIdempotencyKey("activate", project.Name, request.Connection, created.Id)
	activated, err := client.activateRollout(ctx, project.Name, request.Connection, created.Id, activateKey)
	if err != nil {
		return fmt.Errorf("activate managed data rollout failed")
	}
	if err := validateRolloutResponse(activated, created.Id, request.Revision, request.Environment, apigenapi.ManagedDataRolloutStatusActive, apigenapi.ManagedDataRolloutTargetStatusActive, targets); err != nil {
		return err
	}
	_, err = fmt.Fprintf(outputOrDiscard(request.Out), "deployed %s rollout=%s environment=%s status=%s\n", request.Revision, activated.Id, request.Environment, activated.Status)
	return err
}

func affectedManagedDataWorkspaces(project workspacecompiler.Project, connection string) []string {
	ids := make([]string, 0)
	for id, workspaceProject := range project.Workspaces {
		for _, name := range managedConnectionsForWorkspace(project, workspaceProject) {
			if name == connection {
				ids = append(ids, id)
				break
			}
		}
	}
	sort.Strings(ids)
	return ids
}

func otherManagedConnections(project workspacecompiler.Project, workspaceIDs []string, excluded string) []string {
	set := map[string]struct{}{}
	for _, workspaceID := range workspaceIDs {
		for _, connection := range managedConnectionsForWorkspace(project, project.Workspaces[workspaceID]) {
			if connection != excluded {
				set[connection] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(set))
	for connection := range set {
		result = append(result, connection)
	}
	sort.Strings(result)
	return result
}

func validateRolloutResponse(response apigenapi.ManagedDataRolloutResponse, expectedID, revision, environment string, status apigenapi.ManagedDataRolloutStatus, targetStatus apigenapi.ManagedDataRolloutTargetStatus, targets []apigenapi.ManagedDataRolloutTargetRequest) error {
	if strings.TrimSpace(response.Id) == "" || expectedID != "" && response.Id != expectedID || response.RevisionId != revision || response.Environment != environment || response.Status != status || len(response.Targets) != len(targets) {
		return fmt.Errorf("managed data rollout returned inconsistent scope or status")
	}
	expected := map[string]string{}
	for _, target := range targets {
		expected[target.Workspace] = target.ServingStateId
	}
	for _, target := range response.Targets {
		if expected[target.Workspace] != target.ServingStateId || target.Status != targetStatus {
			return fmt.Errorf("managed data rollout returned inconsistent targets")
		}
		delete(expected, target.Workspace)
	}
	if len(expected) != 0 {
		return fmt.Errorf("managed data rollout omitted targets")
	}
	return nil
}

func canonicalArtifactDigest(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func rolloutTargetValues(targets []apigenapi.ManagedDataRolloutTargetRequest) []string {
	values := make([]string, 0, len(targets)*2)
	for _, target := range targets {
		values = append(values, target.Workspace, target.ServingStateId)
	}
	return values
}

func dataDeployIdempotencyKey(kind string, values ...string) string {
	digest := sha256.New()
	writeHashValue(digest, kind)
	for _, value := range values {
		writeHashValue(digest, value)
	}
	return "data-deploy-" + kind + "-" + hex.EncodeToString(digest.Sum(nil))
}

func outputOrDiscard(out io.Writer) io.Writer {
	if out == nil {
		return io.Discard
	}
	return out
}
