package module

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	accesssnapshot "github.com/flidai/leapview/internal/access/snapshot"
	"github.com/flidai/leapview/internal/platform/digest"
	projectartifact "github.com/flidai/leapview/internal/project/artifact"
	projectbundle "github.com/flidai/leapview/internal/project/bundle"
	projectcompiler "github.com/flidai/leapview/internal/project/compiler"
	"github.com/flidai/leapview/internal/release"
	"github.com/flidai/leapview/internal/servingstate"
	servingstatevalidate "github.com/flidai/leapview/internal/servingstate/validate"
	"github.com/flidai/leapview/internal/workspace"
)

type candidateArtifactService struct {
	states      ServingStateRepository
	workspaces  WorkspaceProvisioner
	artifacts   release.ArtifactStore
	validator   servingstatevalidate.Service
	environment servingstate.Environment
}

type candidateWorkspaceBase struct {
	graph      workspace.AssetGraph
	pins       map[string]string
	snapshotID int64
	active     bool
}

func (service *candidateArtifactService) Prepare(
	ctx context.Context,
	request release.CandidateArtifactRequest,
) (release.CandidateArtifactSet, error) {
	request.CandidateID = strings.TrimSpace(request.CandidateID)
	request.ProjectID = strings.TrimSpace(request.ProjectID)
	request.OwnerID = strings.TrimSpace(request.OwnerID)
	request.Environment = strings.TrimSpace(request.Environment)
	request.ArtifactDigest = strings.TrimSpace(request.ArtifactDigest)
	request.Source.ProjectID = strings.TrimSpace(request.Source.ProjectID)
	request.Source.ArtifactDigest = strings.TrimSpace(request.Source.ArtifactDigest)
	request.Source.ProjectPath = strings.TrimSpace(request.Source.ProjectPath)
	request.Source.ProjectDigest = strings.TrimSpace(request.Source.ProjectDigest)
	request.Source.ProjectArtifactPath = strings.TrimSpace(
		request.Source.ProjectArtifactPath,
	)
	if service == nil || service.states == nil || service.workspaces == nil ||
		service.artifacts == nil || request.CandidateID == "" || request.ProjectID == "" ||
		request.OwnerID == "" || request.Environment == "" ||
		request.Source.ProjectArtifactPath == "" ||
		request.Source.ProjectID != request.ProjectID ||
		request.Source.ArtifactDigest != request.ArtifactDigest ||
		digest.ValidateSHA256Identity(request.ArtifactDigest) != nil ||
		digest.ValidateSHA256Identity(request.Source.ProjectDigest) != nil {
		return release.CandidateArtifactSet{}, release.ErrCandidateArtifactInvalid
	}
	environment := servingstate.NormalizeEnvironment(servingstate.Environment(request.Environment))
	if environment != service.environment {
		return release.CandidateArtifactSet{}, fmt.Errorf(
			"%w: candidate environment does not match target",
			release.ErrCandidateArtifactInvalid,
		)
	}
	projectBytes, err := os.ReadFile(request.Source.ProjectArtifactPath)
	if err != nil {
		return release.CandidateArtifactSet{}, candidateArtifactUnavailable(err)
	}
	compiledProject, err := projectartifact.Decode(projectBytes)
	if err != nil {
		return release.CandidateArtifactSet{}, candidateArtifactInvalid(err)
	}
	if compiledProject.ID() != request.ProjectID ||
		compiledProject.Digest() != request.Source.ProjectDigest {
		return release.CandidateArtifactSet{}, candidateArtifactInvalid(
			fmt.Errorf("retained project artifact does not match synchronized project"),
		)
	}
	workspaces := compiledProject.WorkspaceIDs()
	if len(workspaces) == 0 {
		return release.CandidateArtifactSet{}, candidateArtifactInvalid(
			fmt.Errorf("project has no workspaces"),
		)
	}

	result := release.CandidateArtifactSet{
		Workspaces: make([]release.CandidateArtifactWorkspace, 0, len(workspaces)),
	}
	policyHash := sha256.New()
	_, _ = fmt.Fprintf(policyHash, "%d:%s", len(request.OwnerID), request.OwnerID)
	for _, workspaceID := range workspaces {
		if err := ctx.Err(); err != nil {
			return release.CandidateArtifactSet{}, err
		}
		compiledWorkspace, ok := compiledProject.Workspace(workspaceID)
		if !ok {
			return release.CandidateArtifactSet{}, candidateArtifactInvalid(
				fmt.Errorf("project has no workspace %q", workspaceID),
			)
		}
		if err := service.workspaces.Ensure(ctx, workspace.EnsureInput{
			ID: workspace.WorkspaceID(workspaceID), Title: workspaceID,
		}); err != nil {
			return release.CandidateArtifactSet{}, candidateArtifactUnavailable(err)
		}
		base, err := service.workspaceBase(ctx, request.ProjectID, workspaceID, environment)
		if err != nil {
			return release.CandidateArtifactSet{}, err
		}
		workspacePlan, err := projectcompiler.PlanCompiledProjectAgainstGraph(
			compiledProject,
			workspaceID,
			base.graph,
		)
		if err != nil {
			return release.CandidateArtifactSet{}, candidateArtifactInvalid(err)
		}
		reuseSnapshot := base.active && base.snapshotID > 0 &&
			len(workspacePlan.Workspaces) == 1 &&
			!workspacePlan.Workspaces[0].Summary.MaterializationImpact
		requirements, err := candidateConnectionRequirements(compiledWorkspace)
		if err != nil {
			return release.CandidateArtifactSet{}, candidateArtifactInvalid(err)
		}
		if !reuseSnapshot && len(requirements) == 0 {
			return release.CandidateArtifactSet{}, candidateArtifactInvalid(
				fmt.Errorf("workspace %q requires data preparation but has no target connections", workspaceID),
			)
		}
		state, err := service.states.Create(ctx, servingstate.CreateInput{
			WorkspaceID: servingstate.WorkspaceID(workspaceID),
			ProjectID:   request.ProjectID, Environment: environment,
			CreatedBy: request.OwnerID, Source: servingstate.SourceCandidate,
		})
		if err != nil {
			return release.CandidateArtifactSet{}, candidateArtifactUnavailable(err)
		}
		var content bytes.Buffer
		_, expectedDigest, err := projectbundle.PackCompiledProject(
			compiledProject,
			request.ArtifactDigest,
			projectbundle.PackProjectOptions{
				WorkspaceID: workspaceID, Environment: string(environment),
				ServingStateID: string(state.ID), ActiveGraph: base.graph,
				ManagedDataRevisions: base.pins,
			},
			&content,
		)
		if err != nil {
			_ = service.states.MarkFailed(ctx, state.ID, err)
			return release.CandidateArtifactSet{}, candidateArtifactInvalid(err)
		}
		if _, err := service.artifacts.SaveUpload(ctx, state.ID, &content); err != nil {
			_ = service.states.MarkFailed(ctx, state.ID, err)
			return release.CandidateArtifactSet{}, candidateArtifactUnavailable(err)
		}
		validated, err := service.validator.Validate(ctx, state.ID)
		if err != nil {
			return release.CandidateArtifactSet{}, candidateArtifactInvalid(err)
		}
		if validated.Digest != expectedDigest {
			return release.CandidateArtifactSet{}, candidateArtifactInvalid(
				fmt.Errorf("candidate artifact digest changed during validation"),
			)
		}
		restrictions, err := candidateRestrictions(
			validated.AccessPolicyJSON,
			workspaceID,
			request.OwnerID,
		)
		if err != nil {
			return release.CandidateArtifactSet{}, candidateArtifactInvalid(err)
		}
		mode := "refresh_sources"
		dataRevision := "sources:" + request.ArtifactDigest
		connections := requirements
		if reuseSnapshot {
			if err := service.states.RecordDuckLakeSnapshot(
				ctx,
				validated.ID,
				base.snapshotID,
			); err != nil {
				return release.CandidateArtifactSet{}, candidateArtifactUnavailable(err)
			}
			mode = "reuse_snapshot"
			dataRevision = fmt.Sprintf("snapshot:%d", base.snapshotID)
			connections = nil
		}
		result.Workspaces = append(result.Workspaces, release.CandidateArtifactWorkspace{
			WorkspaceID: workspaceID, ServingStateID: string(validated.ID),
			ArtifactDigest: validated.Digest, DataRevision: dataRevision,
			DataMode: mode, Connections: connections,
			Restrictions: restrictions,
		})
		_, _ = fmt.Fprintf(
			policyHash,
			"%d:%s:%d:%s",
			len(workspaceID),
			workspaceID,
			len(validated.AccessPolicyJSON),
			validated.AccessPolicyJSON,
		)
	}
	result.AuthorizationFingerprint = "sha256:" + hex.EncodeToString(policyHash.Sum(nil))
	return result, nil
}

func candidateRestrictions(policyJSON, workspaceID, ownerID string) ([]release.CandidateRestriction, error) {
	policy, err := accesssnapshot.Decode([]byte(policyJSON))
	if err != nil {
		return nil, fmt.Errorf("decode candidate access policy: %w", err)
	}
	names := make([]string, 0, len(policy.DataPolicies))
	for name := range policy.DataPolicies {
		names = append(names, name)
	}
	sort.Strings(names)
	restrictions := make([]release.CandidateRestriction, 0, len(names))
	for _, name := range names {
		item := policy.DataPolicies[name]
		applies, err := candidateSubjectApplies(policy, item.Subject, ownerID)
		if err != nil {
			return nil, fmt.Errorf("candidate data policy %q: %w", name, err)
		}
		if !applies {
			continue
		}
		objectID := "workspace:" + workspaceID
		if item.Object.Type != "workspace" {
			objectID = item.Object.Type + ":" + workspaceID + ":" + item.Object.ID
		}
		restrictions = append(restrictions, release.CandidateRestriction{
			ID: firstCandidateValue(item.ID, name), WorkspaceID: workspaceID,
			ObjectID: objectID, PolicyType: item.PolicyType,
			ExpressionJSON: item.ExpressionJSON,
		})
	}
	return restrictions, nil
}

func candidateSubjectApplies(
	policy accesssnapshot.AccessPolicy,
	subject accesssnapshot.Subject,
	ownerID string,
) (bool, error) {
	switch subject.Kind {
	case "":
		return true, nil
	case "principal", "service_principal":
		if subject.PrincipalID == "" {
			return false, fmt.Errorf("email-only subjects cannot be resolved in a private candidate")
		}
		return subject.PrincipalID == ownerID, nil
	case "group":
		group, ok := policy.Groups[subject.Group]
		if !ok {
			return false, fmt.Errorf("unknown group %q", subject.Group)
		}
		for _, member := range group.Members {
			if member.PrincipalID == ownerID {
				return true, nil
			}
			if member.PrincipalID == "" && member.Email != "" {
				return false, fmt.Errorf("email-only group members cannot be resolved in a private candidate")
			}
		}
		return false, nil
	default:
		return false, nil
	}
}

func firstCandidateValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func (service *candidateArtifactService) workspaceBase(
	ctx context.Context,
	projectID, workspaceID string,
	environment servingstate.Environment,
) (candidateWorkspaceBase, error) {
	state, artifact, err := service.states.ActiveArtifact(
		ctx,
		servingstate.WorkspaceID(workspaceID),
		environment,
	)
	if errors.Is(err, servingstate.ErrNotFound) {
		return candidateWorkspaceBase{
			graph: workspace.AssetGraph{}, pins: map[string]string{},
		}, nil
	}
	if err != nil {
		return candidateWorkspaceBase{}, candidateArtifactUnavailable(err)
	}
	if state.ProjectID != projectID || artifact.Path == "" {
		return candidateWorkspaceBase{}, candidateArtifactInvalid(
			fmt.Errorf("active workspace belongs to a different project"),
		)
	}
	validation, err := projectbundle.ValidateArtifactWithOptions(
		artifact.Path,
		workspaceID,
		string(state.ID),
		projectbundle.ValidateOptions{Environment: string(environment)},
	)
	if err != nil {
		return candidateWorkspaceBase{}, candidateArtifactUnavailable(err)
	}
	if validation.RootDir != "" {
		defer os.RemoveAll(validation.RootDir)
	}
	return candidateWorkspaceBase{
		graph: validation.Graph, pins: cloneCandidatePins(validation.ManagedDataRevisions),
		snapshotID: state.DuckLakeSnapshotID, active: true,
	}, nil
}

func candidateConnectionRequirements(
	compiled projectartifact.Workspace,
) ([]release.CandidateConnectionRequirement, error) {
	definition := compiled.Manifest()
	if definition == nil {
		return nil, fmt.Errorf("compiled workspace definition is required")
	}
	kinds := map[string]string{}
	for _, model := range definition.Models {
		if model == nil {
			return nil, fmt.Errorf("compiled workspace contains a nil semantic model")
		}
		for connectionID, connection := range model.Connections {
			kind := strings.TrimSpace(connection.Kind)
			if connectionID == "" || kind == "" {
				return nil, fmt.Errorf("compiled workspace contains invalid connection metadata")
			}
			if existing, ok := kinds[connectionID]; ok && existing != kind {
				return nil, fmt.Errorf(
					"compiled workspace connection %q has conflicting connector kinds",
					connectionID,
				)
			}
			kinds[connectionID] = kind
		}
	}
	connectionIDs := make([]string, 0, len(kinds))
	for connectionID := range kinds {
		connectionIDs = append(connectionIDs, connectionID)
	}
	sort.Strings(connectionIDs)
	requirements := make(
		[]release.CandidateConnectionRequirement,
		0,
		len(connectionIDs),
	)
	for _, connectionID := range connectionIDs {
		requirements = append(requirements, release.CandidateConnectionRequirement{
			LogicalConnectionID: connectionID,
			ConnectorKind:       kinds[connectionID],
		})
	}
	return requirements, nil
}

func cloneCandidatePins(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func candidateArtifactInvalid(err error) error {
	return fmt.Errorf("%w: %v", release.ErrCandidateArtifactInvalid, err)
}

func candidateArtifactUnavailable(err error) error {
	return fmt.Errorf("%w: %v", release.ErrCandidateArtifactUnavailable, err)
}
