package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	accesssnapshot "github.com/Yacobolo/leapview/internal/access/snapshot"
	refreshartifact "github.com/Yacobolo/leapview/internal/refresh/artifact"
	refreshplan "github.com/Yacobolo/leapview/internal/refresh/plan"
	refreshschedule "github.com/Yacobolo/leapview/internal/refresh/schedule"
	servingstate "github.com/Yacobolo/leapview/internal/servingstate"
	servingstatevalidation "github.com/Yacobolo/leapview/internal/servingstate/validation"
	"github.com/Yacobolo/leapview/internal/workspace"
)

type ServingStateRepository interface {
	ActiveArtifact(ctx context.Context, workspaceID servingstate.WorkspaceID, environment servingstate.Environment) (servingstate.State, servingstate.Artifact, error)
	Create(ctx context.Context, input servingstate.CreateInput) (servingstate.State, error)
	SaveValidated(ctx context.Context, servingStateID servingstate.ID, validation servingstate.Validation, artifact servingstate.Artifact) (servingstate.State, error)
	ByID(ctx context.Context, id servingstate.ID) (servingstate.State, error)
	ArtifactByServingState(ctx context.Context, servingStateID servingstate.ID) (servingstate.Artifact, error)
	RecordDuckLakeSnapshot(ctx context.Context, servingStateID servingstate.ID, snapshotID int64) error
	Activate(ctx context.Context, workspaceID servingstate.WorkspaceID, environment servingstate.Environment, servingStateID servingstate.ID) (servingstate.State, error)
	MarkFailed(ctx context.Context, servingStateID servingstate.ID, cause error) error
}

type WorkflowRepository interface {
	CreateRun(ctx context.Context, input RunInput) (RunRecord, error)
	ListChildRuns(ctx context.Context, workspaceID, parentRunID string) ([]RunRecord, error)
	MarkRunRunning(ctx context.Context, workspaceID, runID string) (RunRecord, error)
	MarkRunSucceeded(ctx context.Context, workspaceID, runID string) (RunRecord, error)
	MarkRunFailed(ctx context.Context, workspaceID, runID, message string) (RunRecord, error)
	MarkRunPrepared(ctx context.Context, job JobRecord) (RunRecord, error)
	RunMayPublish(ctx context.Context, job JobRecord) (bool, error)
}

type LoadedArtifact struct {
	Definition           *refreshartifact.Definition
	Graph                workspace.AssetGraph
	ManagedDataRevisions map[string]string
}

type ArtifactLoader interface {
	Load(ctx context.Context, artifact servingstate.Artifact) (LoadedArtifact, error)
}

type Materializer interface {
	Materialize(ctx context.Context, input MaterializeInput) (int64, error)
}

type MaterializeInput struct {
	Definition  *refreshartifact.Definition
	Active      servingstate.State
	Candidate   servingstate.State
	Artifact    servingstate.Artifact
	Environment servingstate.Environment
	Plan        refreshplan.Plan
}

type RuntimeHost interface {
	PrepareServingState(ctx context.Context, servingStateID string) (servingstate.PreparedRuntime, error)
	ActivatePrepared(prepared servingstate.PreparedRuntime, activate func() error) error
}

type RetentionRunner interface {
	Run(ctx context.Context, dryRun bool) error
}

type Publisher interface {
	PublishRefreshTarget(ctx context.Context, workspaceID, environment, targetType, targetID string)
}

type DataVersionRepository interface {
	SaveDataVersion(context.Context, refreshschedule.DataVersion) error
}

type CandidateValidationHook interface {
	AfterArtifactValidation(context.Context, servingstate.State, servingstate.Validation) error
}

type PublicationUnitOfWork interface {
	Publish(context.Context, servingstate.WorkspaceID, servingstate.Environment, servingstate.ID, refreshschedule.DataVersion) error
}

type Service struct {
	ServingStates            ServingStateRepository
	Runs                     WorkflowRepository
	Artifacts                ArtifactLoader
	Materializer             Materializer
	Runtime                  RuntimeHost
	Retention                RetentionRunner
	Publisher                Publisher
	DataVersions             DataVersionRepository
	Publication              PublicationUnitOfWork
	CandidateValidationHooks []CandidateValidationHook
	Now                      func() time.Time
}

type ServingState struct {
	State    servingstate.State
	Artifact servingstate.Artifact
}

type QueueAssetResult struct {
	Run            RunRecord
	DependencyRuns []RunRecord
	ServingStateID servingstate.ID
}

type QueuePipelineInput struct {
	WorkspaceID    string
	Environment    servingstate.Environment
	PrincipalID    string
	PipelineID     string
	TriggerType    string
	RetryOf        string
	ArtifactDigest string
	Occurrence     *refreshschedule.Occurrence
}

func (s Service) QueuePipelineRefresh(ctx context.Context, input QueuePipelineInput) (QueueAssetResult, error) {
	if s.ServingStates == nil || s.Runs == nil || s.Artifacts == nil {
		return QueueAssetResult{}, fmt.Errorf("serving state, refresh run, and artifact repositories are required")
	}
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.PipelineID = strings.TrimSpace(input.PipelineID)
	if input.WorkspaceID == "" || input.PipelineID == "" {
		return QueueAssetResult{}, fmt.Errorf("workspace id and pipeline id are required")
	}
	if input.TriggerType == "" {
		input.TriggerType = TriggerManual
	}
	switch input.TriggerType {
	case TriggerManual, TriggerSchedule, TriggerRetry:
	default:
		return QueueAssetResult{}, fmt.Errorf("unsupported refresh pipeline trigger %q", input.TriggerType)
	}
	targetID := input.WorkspaceID + "." + input.PipelineID
	environment := servingstate.NormalizeEnvironment(input.Environment)
	active, err := s.Active(ctx, input.WorkspaceID, environment)
	if err != nil {
		return QueueAssetResult{}, err
	}
	if input.ArtifactDigest != "" && input.ArtifactDigest != active.Artifact.Digest {
		return QueueAssetResult{}, fmt.Errorf("refresh pipeline schedule belongs to superseded artifact %q", input.ArtifactDigest)
	}
	loaded, err := s.Artifacts.Load(ctx, active.Artifact)
	if err != nil {
		return QueueAssetResult{}, err
	}
	if loaded.Definition == nil {
		return QueueAssetResult{}, fmt.Errorf("compiled workspace definition is required")
	}
	pipeline, ok := loaded.Definition.Pipelines[input.PipelineID]
	if !ok {
		return QueueAssetResult{}, fmt.Errorf("unknown refresh pipeline %q", input.PipelineID)
	}
	plan, err := refreshplan.ForPipeline(loaded.Definition, input.WorkspaceID, input.PipelineID)
	if err != nil {
		return QueueAssetResult{}, err
	}
	candidate, err := s.CreateRefreshCandidate(ctx, RefreshCandidateInput{
		WorkspaceID: input.WorkspaceID, Environment: environment, CreatedBy: input.PrincipalID,
		Active: active, ArtifactGraph: loaded.Graph, ManagedDataRevisions: loaded.ManagedDataRevisions,
	})
	if err != nil {
		return QueueAssetResult{}, err
	}
	payload, _ := json.Marshal(map[string]string{"pipelineId": input.PipelineID, "semanticModel": pipeline.SemanticModel})
	rootInput := RunInput{
		WorkspaceID: input.WorkspaceID, Environment: string(environment), ModelID: pipeline.SemanticModel, ServingStateID: string(candidate.State.ID),
		PrincipalID: input.PrincipalID, TargetType: TargetRefreshPipeline, TargetID: targetID,
		TriggerType: input.TriggerType, RetryOf: input.RetryOf, JobKind: JobKindRefreshPipeline,
		PayloadJSON: string(payload),
	}
	var rootRun RunRecord
	if input.Occurrence != nil {
		creator, ok := s.Runs.(interface {
			CreateScheduledRun(context.Context, RunInput, refreshschedule.Occurrence) (RunRecord, error)
		})
		if !ok {
			err = fmt.Errorf("refresh run repository does not support atomic scheduled runs")
		} else {
			rootRun, err = creator.CreateScheduledRun(ctx, rootInput, *input.Occurrence)
		}
	} else {
		rootRun, err = s.Runs.CreateRun(ctx, rootInput)
	}
	if err != nil {
		_ = s.MarkFailed(ctx, candidate, err)
		if active, found, lookupErr := latestActivePipelineRun(ctx, s.Runs, input.WorkspaceID, string(environment), targetID); lookupErr == nil && found {
			return QueueAssetResult{Run: active, ServingStateID: servingstate.ID(active.ServingStateID)}, nil
		}
		return QueueAssetResult{}, err
	}
	children := make([]RunRecord, 0, len(plan.DependencyTables))
	for _, table := range plan.DependencyTables {
		run, err := s.Runs.CreateRun(ctx, RunInput{
			WorkspaceID: input.WorkspaceID, Environment: string(environment), ModelID: pipeline.SemanticModel, ServingStateID: string(candidate.State.ID),
			PrincipalID: input.PrincipalID, TargetType: TargetModelTable, TargetID: input.WorkspaceID + "." + table,
			TargetGeneration: rootRun.TargetGeneration, TriggerType: TriggerDependency, ParentRunID: rootRun.ID, JobKind: JobKindChildRun,
		})
		if err != nil {
			_, _ = s.Runs.MarkRunFailed(ctx, input.WorkspaceID, rootRun.ID, err.Error())
			_ = s.MarkFailed(ctx, candidate, err)
			return QueueAssetResult{}, err
		}
		children = append(children, run)
	}
	s.publish(ctx, input.WorkspaceID, string(environment), TargetRefreshPipeline, targetID)
	return QueueAssetResult{Run: rootRun, DependencyRuns: children, ServingStateID: candidate.State.ID}, nil
}

func latestActivePipelineRun(ctx context.Context, runs WorkflowRepository, workspaceID, environment, targetID string) (RunRecord, bool, error) {
	repository, ok := runs.(interface {
		LatestTargetRun(context.Context, string, string, string, string) (RunRecord, bool, error)
	})
	if !ok {
		return RunRecord{}, false, nil
	}
	run, found, err := repository.LatestTargetRun(ctx, workspaceID, environment, TargetRefreshPipeline, targetID)
	if err != nil || !found {
		return run, false, err
	}
	return run, run.Status == RunStatusQueued || run.Status == RunStatusRunning, nil
}

func (s Service) ExecuteClaimedJob(ctx context.Context, job JobRecord) error {
	if s.ServingStates == nil {
		return fmt.Errorf("serving state repository is required")
	}
	if s.Runs == nil {
		return fmt.Errorf("refresh run repository is required")
	}
	if s.Artifacts == nil {
		return fmt.Errorf("artifact loader is required")
	}
	if s.Materializer == nil {
		return fmt.Errorf("refresh materializer is required")
	}
	if job.ServingStateID == "" {
		return fmt.Errorf("workspace refresh job serving state id is required")
	}
	candidateState, err := s.ServingStates.ByID(ctx, servingstate.ID(job.ServingStateID))
	if err != nil {
		return err
	}
	if candidateState.Status == servingstate.StatusActive && candidateState.DuckLakeSnapshotID > 0 {
		_, _ = s.Runs.MarkRunSucceeded(ctx, job.WorkspaceID, job.RunID)
		return nil
	}
	candidateArtifact, err := s.ServingStates.ArtifactByServingState(ctx, candidateState.ID)
	if err != nil {
		return err
	}
	activeState, err := s.Active(ctx, job.WorkspaceID, candidateState.Environment)
	if err != nil {
		return err
	}
	loaded, err := s.Artifacts.Load(ctx, candidateArtifact)
	if err != nil {
		return err
	}
	if loaded.Definition == nil {
		return fmt.Errorf("compiled workspace definition is required")
	}
	pipelineID := strings.TrimPrefix(job.TargetID, job.WorkspaceID+".")
	plan, err := refreshplan.ForPipeline(loaded.Definition, job.WorkspaceID, pipelineID)
	if err != nil {
		return err
	}
	childRuns, err := s.Runs.ListChildRuns(ctx, job.WorkspaceID, job.RunID)
	if err != nil {
		return err
	}
	for _, child := range childRuns {
		_, _ = s.Runs.MarkRunRunning(ctx, job.WorkspaceID, child.ID)
	}
	s.publish(ctx, job.WorkspaceID, string(candidateState.Environment), job.TargetType, job.TargetID)
	candidate := ServingState{State: candidateState, Artifact: candidateArtifact}
	snapshotID, err := s.Materializer.Materialize(ctx, MaterializeInput{
		Definition:  loaded.Definition,
		Active:      activeState.State,
		Candidate:   candidate.State,
		Artifact:    candidate.Artifact,
		Environment: candidateState.Environment,
		Plan:        plan,
	})
	if err != nil {
		s.failJob(ctx, job, childRuns, candidate, err)
		return err
	}
	if err := s.RecordSnapshot(ctx, candidate, snapshotID); err != nil {
		s.failJob(ctx, job, childRuns, candidate, err)
		return err
	}
	if s.Runtime == nil {
		err = fmt.Errorf("runtime host is required for refresh activation")
		s.failJob(ctx, job, childRuns, candidate, err)
		return err
	}
	prepared, err := s.Runtime.PrepareServingState(ctx, string(candidateState.ID))
	if err != nil {
		s.failJob(ctx, job, childRuns, candidate, err)
		return err
	}
	if prepared == nil {
		err = fmt.Errorf("runtime host returned a nil prepared runtime")
		s.failJob(ctx, job, childRuns, candidate, err)
		return err
	}
	if _, err := s.Runs.MarkRunPrepared(ctx, job); err != nil {
		_ = prepared.Close()
		return err
	}
	mayPublish, err := s.Runs.RunMayPublish(ctx, job)
	if err != nil {
		_ = prepared.Close()
		return err
	}
	if !mayPublish {
		_ = prepared.Close()
		return ErrLeaseLost
	}
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	dataVersion := refreshschedule.DataVersion{
		WorkspaceID: job.WorkspaceID, Environment: string(candidateState.Environment), SemanticModel: job.ModelID,
		SnapshotID: snapshotID, ServingStateID: string(candidateState.ID), RefreshedAt: now.UTC(),
		Source: refreshschedule.DataVersionSourceRefresh, PipelineID: pipelineID, RunID: job.RunID,
		TargetGeneration: job.TargetGeneration, LeaseOwner: job.LeaseOwner, LeaseGeneration: job.LeaseGeneration,
	}
	activate := func() error { return s.activateRefresh(ctx, candidate, dataVersion) }
	err = s.Runtime.ActivatePrepared(prepared, activate)
	if err != nil {
		_ = prepared.Close()
		if errors.Is(err, ErrLeaseLost) {
			return err
		}
		s.failJob(ctx, job, childRuns, candidate, err)
		return err
	}
	if job.TargetType == TargetRefreshPipeline && s.DataVersions != nil {
		if publisher, ok := s.Publisher.(interface {
			PublishSemanticModelVersion(context.Context, string, string, string)
		}); ok {
			publisher.PublishSemanticModelVersion(ctx, job.WorkspaceID, string(candidateState.Environment), job.ModelID)
		}
	}
	if s.Retention != nil {
		_ = s.Retention.Run(ctx, false)
	}
	for _, child := range childRuns {
		_, _ = s.Runs.MarkRunSucceeded(ctx, job.WorkspaceID, child.ID)
	}
	s.publish(ctx, job.WorkspaceID, string(candidateState.Environment), job.TargetType, job.TargetID)
	return err
}

func (s Service) activateRefresh(ctx context.Context, candidate ServingState, version refreshschedule.DataVersion) error {
	if s.Publication == nil {
		return fmt.Errorf("refresh publication unit of work is required")
	}
	return s.Publication.Publish(ctx, candidate.State.WorkspaceID, candidate.State.Environment, candidate.State.ID, version)
}

func (s Service) failJob(ctx context.Context, job JobRecord, childRuns []RunRecord, candidate ServingState, cause error) {
	_, _ = s.Runs.MarkRunFailed(ctx, job.WorkspaceID, job.RunID, cause.Error())
	for _, child := range childRuns {
		_, _ = s.Runs.MarkRunFailed(ctx, job.WorkspaceID, child.ID, cause.Error())
	}
	_ = s.MarkFailed(ctx, candidate, cause)
	s.publish(ctx, job.WorkspaceID, string(candidate.State.Environment), job.TargetType, job.TargetID)
}

func (s Service) publish(ctx context.Context, workspaceID, environment, targetType, targetID string) {
	if s.Publisher != nil {
		s.Publisher.PublishRefreshTarget(ctx, workspaceID, environment, targetType, targetID)
	}
}

type RefreshCandidateInput struct {
	WorkspaceID          string
	Environment          servingstate.Environment
	CreatedBy            string
	Active               ServingState
	ArtifactGraph        workspace.AssetGraph
	ManagedDataRevisions map[string]string
}

func (s Service) Active(ctx context.Context, workspaceID string, environment servingstate.Environment) (ServingState, error) {
	active, artifact, err := s.ServingStates.ActiveArtifact(ctx, servingstate.WorkspaceID(workspaceID), environment)
	if err != nil {
		return ServingState{}, err
	}
	return ServingState{State: active, Artifact: artifact}, nil
}

func (s Service) CreateRefreshCandidate(ctx context.Context, input RefreshCandidateInput) (ServingState, error) {
	active := input.Active
	workspaceID := servingstate.WorkspaceID(input.WorkspaceID)
	environment := servingstate.NormalizeEnvironment(input.Environment)
	accessPolicy, err := accesssnapshot.Decode([]byte(active.State.AccessPolicyJSON))
	if err != nil {
		return ServingState{}, fmt.Errorf("decode active access policy: %w", err)
	}
	created, err := s.ServingStates.Create(ctx, servingstate.CreateInput{
		WorkspaceID: workspaceID,
		ProjectID:   active.State.ProjectID,
		Environment: environment,
		CreatedBy:   input.CreatedBy,
		Source:      servingstate.SourceRefresh,
	})
	if err != nil {
		return ServingState{}, err
	}
	candidateArtifact := servingstate.Artifact{
		ID:             "artifact_" + string(created.ID),
		ServingStateID: created.ID,
		WorkspaceID:    workspaceID,
		Environment:    environment,
		Digest:         active.Artifact.Digest,
		Format:         active.Artifact.Format,
		Path:           active.Artifact.Path,
		ManifestJSON:   active.Artifact.ManifestJSON,
		SizeBytes:      active.Artifact.SizeBytes,
		CreatedAt:      active.Artifact.CreatedAt,
	}
	graphJSON, err := json.Marshal(RetargetAssetGraph(input.ArtifactGraph, workspace.WorkspaceID(input.WorkspaceID), workspace.ServingStateID(created.ID)))
	if err != nil {
		return ServingState{}, fmt.Errorf("encode refresh asset snapshot: %w", err)
	}
	graph, err := servingstatevalidation.DecodeAssetGraph(graphJSON)
	if err != nil {
		return ServingState{}, fmt.Errorf("decode refresh asset snapshot: %w", err)
	}
	validation := servingstate.Validation{
		Digest:               active.State.Digest,
		ManifestJSON:         active.State.ManifestJSON,
		ProjectID:            active.State.ProjectID,
		ProjectDigest:        active.State.ProjectDigest,
		ProjectWorkspaces:    append([]string(nil), active.State.ProjectWorkspaces...),
		AccessPolicy:         accessPolicy,
		ManagedDataRevisions: cloneStringMap(input.ManagedDataRevisions),
		Graph:                graph,
	}
	for _, hook := range s.CandidateValidationHooks {
		if hook == nil {
			continue
		}
		if err := hook.AfterArtifactValidation(ctx, created, validation); err != nil {
			_ = s.ServingStates.MarkFailed(ctx, created.ID, err)
			return ServingState{}, err
		}
	}
	validated, err := s.ServingStates.SaveValidated(ctx, created.ID, validation, candidateArtifact)
	if err != nil {
		_ = s.ServingStates.MarkFailed(ctx, created.ID, err)
		return ServingState{}, err
	}
	return ServingState{State: validated, Artifact: candidateArtifact}, nil
}

func cloneStringMap(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func (s Service) RecordSnapshot(ctx context.Context, candidate ServingState, snapshotID int64) error {
	if snapshotID <= 0 {
		return fmt.Errorf("serving state snapshot id must be positive")
	}
	return s.ServingStates.RecordDuckLakeSnapshot(ctx, candidate.State.ID, snapshotID)
}

func (s Service) Activate(ctx context.Context, candidate ServingState) (servingstate.State, error) {
	return s.ServingStates.Activate(ctx, candidate.State.WorkspaceID, candidate.State.Environment, candidate.State.ID)
}

func (s Service) MarkFailed(ctx context.Context, state ServingState, cause error) error {
	if state.State.ID == "" || cause == nil {
		return nil
	}
	return s.ServingStates.MarkFailed(ctx, state.State.ID, cause)
}

func RetargetAssetGraph(graph workspace.AssetGraph, workspaceID workspace.WorkspaceID, servingStateID workspace.ServingStateID) workspace.AssetGraph {
	out := workspace.AssetGraph{
		Assets: make([]workspace.Asset, 0, len(graph.Assets)),
		Edges:  make([]workspace.AssetEdge, 0, len(graph.Edges)),
	}
	for _, asset := range graph.Assets {
		asset.WorkspaceID = workspaceID
		asset.ServingStateID = servingStateID
		asset.SnapshotID = workspace.NewAssetSnapshotID(servingStateID, asset.ID)
		out.Assets = append(out.Assets, asset)
	}
	for _, edge := range graph.Edges {
		edge.WorkspaceID = workspaceID
		edge.ServingStateID = servingStateID
		edge.ID = workspace.NewAssetEdgeID(servingStateID, edge.FromAssetID, edge.ToAssetID, edge.Type)
		out.Edges = append(out.Edges, edge)
	}
	return out
}
