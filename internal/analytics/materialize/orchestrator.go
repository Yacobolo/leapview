package materialize

import (
	"context"
	"errors"
	"fmt"
	"strings"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
)

type RefreshRunner interface {
	RefreshMaterializations(ctx context.Context, modelID string) error
	RefreshModelTables(ctx context.Context, modelID string, tableNames []string) error
}

type ModelLookup func(string) (*semanticmodel.Model, bool)

type RefreshPublisher struct {
	Root   func()
	Target func(targetID string)
}

func (p RefreshPublisher) publishRoot() {
	if p.Root != nil {
		p.Root()
	}
}

func (p RefreshPublisher) publishTarget(targetID string) {
	if p.Target != nil {
		p.Target(targetID)
	}
}

type RefreshOrchestrator struct {
	repo                RunRepository
	runner              RefreshRunner
	model               ModelLookup
	allowDirectFallback bool
}

func NewRefreshOrchestrator(repo RunRepository, runner RefreshRunner, model ModelLookup) RefreshOrchestrator {
	return RefreshOrchestrator{repo: repo, runner: runner, model: model}
}

func NewGenericRefreshOrchestrator(repo RunRepository, runner RefreshRunner, model ModelLookup) RefreshOrchestrator {
	orchestrator := NewRefreshOrchestrator(repo, runner, model)
	orchestrator.allowDirectFallback = true
	return orchestrator
}

type RefreshRunInput struct {
	WorkspaceID string
	ModelID     string
	PrincipalID string
	TargetID    string
}

func (o RefreshOrchestrator) RefreshSemanticModel(ctx context.Context, input RefreshRunInput, publisher RefreshPublisher) error {
	run, err := o.repo.CreateRun(ctx, RunInput{
		WorkspaceID: input.WorkspaceID,
		ModelID:     input.ModelID,
		PrincipalID: input.PrincipalID,
		TargetType:  TargetSemanticModel,
		TargetID:    firstNonEmptyString(input.TargetID, input.ModelID),
		TriggerType: TriggerDirect,
	})
	if err != nil {
		return err
	}
	_, err = o.executeSemanticModelRun(ctx, input.WorkspaceID, run, input.PrincipalID, publisher)
	return err
}

func (o RefreshOrchestrator) ExecuteRun(ctx context.Context, workspaceID, runID string, publisher RefreshPublisher) (RunRecord, error) {
	run, err := o.repo.GetRun(ctx, workspaceID, runID)
	if err != nil {
		return RunRecord{}, err
	}
	var finished RunRecord
	if run.TargetType == TargetModelTable {
		finished, err = o.executeModelTableRun(ctx, workspaceID, run, publisher)
	} else {
		finished, err = o.executeSemanticModelRun(ctx, workspaceID, run, run.PrincipalID, publisher)
	}
	if err == nil {
		return finished, nil
	}
	if stored, getErr := o.repo.GetRun(ctx, workspaceID, run.ID); getErr == nil && stored.Status == RunStatusFailed {
		return stored, err
	}
	failed, finishErr := o.repo.MarkRunFailed(ctx, workspaceID, run.ID, err.Error())
	if finishErr != nil {
		return failed, finishErr
	}
	o.publishRunFailure(run, publisher)
	return failed, err
}

func (o RefreshOrchestrator) executeSemanticModelRun(ctx context.Context, workspaceID string, run RunRecord, principalID string, publisher RefreshPublisher) (RunRecord, error) {
	publisher.publishRoot()
	if _, err := o.repo.MarkRunRunning(ctx, workspaceID, run.ID); err != nil {
		return RunRecord{}, err
	}
	publisher.publishRoot()

	if o.model == nil {
		if o.allowDirectFallback {
			return o.executeSemanticModelDirectRun(ctx, workspaceID, run, publisher.publishRoot)
		}
		return RunRecord{}, o.failRun(ctx, workspaceID, run.ID, errors.New("semantic model lookup is not configured"), publisher.publishRoot)
	}
	model, ok := o.model(run.ModelID)
	if !ok {
		if o.allowDirectFallback {
			return o.executeSemanticModelDirectRun(ctx, workspaceID, run, publisher.publishRoot)
		}
		return RunRecord{}, o.failRun(ctx, workspaceID, run.ID, fmt.Errorf("unknown semantic model %q", run.ModelID), publisher.publishRoot)
	}
	order, err := ModelTableOrder(model)
	if err != nil {
		return RunRecord{}, o.failRun(ctx, workspaceID, run.ID, err, publisher.publishRoot)
	}
	for _, tableName := range order {
		if err := o.refreshChildTable(ctx, workspaceID, run.ModelID, tableName, TriggerSemanticModel, run.ID, principalID, publisher); err != nil {
			return RunRecord{}, o.failRun(ctx, workspaceID, run.ID, err, publisher.publishRoot)
		}
	}
	finished, err := o.repo.MarkRunSucceeded(ctx, workspaceID, run.ID)
	if err != nil {
		return RunRecord{}, err
	}
	publisher.publishRoot()
	return finished, nil
}

func (o RefreshOrchestrator) executeSemanticModelDirectRun(ctx context.Context, workspaceID string, run RunRecord, publish func()) (RunRecord, error) {
	if err := o.runner.RefreshMaterializations(ctx, run.ModelID); err != nil {
		failed, finishErr := o.repo.MarkRunFailed(ctx, workspaceID, run.ID, err.Error())
		if finishErr != nil {
			return failed, finishErr
		}
		if publish != nil {
			publish()
		}
		return failed, err
	}
	finished, err := o.repo.MarkRunSucceeded(ctx, workspaceID, run.ID)
	if err != nil {
		return RunRecord{}, err
	}
	if publish != nil {
		publish()
	}
	return finished, nil
}

func (o RefreshOrchestrator) RefreshModelTable(ctx context.Context, input RefreshRunInput, tableName string, publisher RefreshPublisher) error {
	if o.model == nil {
		return errors.New("semantic model lookup is not configured")
	}
	model, ok := o.model(input.ModelID)
	if !ok {
		return fmt.Errorf("unknown semantic model %q", input.ModelID)
	}
	if _, err := ModelTableDependencyOrder(model, tableName); err != nil {
		return err
	}
	root, err := o.repo.CreateRun(ctx, RunInput{
		WorkspaceID: input.WorkspaceID,
		ModelID:     input.ModelID,
		PrincipalID: input.PrincipalID,
		TargetType:  TargetModelTable,
		TargetID:    input.TargetID,
		TriggerType: TriggerDirect,
	})
	if err != nil {
		return err
	}
	_, err = o.executeModelTableDirectRun(ctx, input.WorkspaceID, root, tableName, publisher)
	return err
}

func (o RefreshOrchestrator) executeModelTableRun(ctx context.Context, workspaceID string, run RunRecord, publisher RefreshPublisher) (RunRecord, error) {
	tableName, err := tableNameFromTargetID(run.ModelID, run.TargetID)
	if err != nil {
		return RunRecord{}, err
	}
	if run.TriggerType == TriggerDirect && run.ParentRunID == "" {
		return o.executeModelTableDirectRun(ctx, workspaceID, run, tableName, publisher)
	}
	return o.executeSingleModelTableRun(ctx, workspaceID, run, tableName, publisher.publishTarget)
}

func (o RefreshOrchestrator) executeModelTableDirectRun(ctx context.Context, workspaceID string, root RunRecord, tableName string, publisher RefreshPublisher) (RunRecord, error) {
	publisher.publishRoot()

	if o.model == nil {
		if o.allowDirectFallback {
			return o.executeRootModelTableRun(ctx, workspaceID, root, tableName, publisher.publishRoot)
		}
		return RunRecord{}, o.failRun(ctx, workspaceID, root.ID, errors.New("semantic model lookup is not configured"), publisher.publishRoot)
	}
	model, ok := o.model(root.ModelID)
	if !ok {
		if o.allowDirectFallback {
			return o.executeRootModelTableRun(ctx, workspaceID, root, tableName, publisher.publishRoot)
		}
		return RunRecord{}, o.failRun(ctx, workspaceID, root.ID, fmt.Errorf("unknown semantic model %q", root.ModelID), publisher.publishRoot)
	}
	order, err := ModelTableDependencyOrder(model, tableName)
	if err != nil {
		return RunRecord{}, o.failRun(ctx, workspaceID, root.ID, err, publisher.publishRoot)
	}
	dependencies := order[:len(order)-1]
	for _, dependency := range dependencies {
		if err := o.refreshChildTable(ctx, workspaceID, root.ModelID, dependency, TriggerDependency, root.ID, root.PrincipalID, publisher); err != nil {
			return RunRecord{}, o.failRun(ctx, workspaceID, root.ID, err, publisher.publishRoot)
		}
	}
	if _, err := o.repo.MarkRunRunning(ctx, workspaceID, root.ID); err != nil {
		return RunRecord{}, err
	}
	publisher.publishRoot()
	if err := o.runner.RefreshModelTables(ctx, root.ModelID, []string{tableName}); err != nil {
		return RunRecord{}, o.failRun(ctx, workspaceID, root.ID, err, publisher.publishRoot)
	}
	finished, err := o.repo.MarkRunSucceeded(ctx, workspaceID, root.ID)
	if err != nil {
		return RunRecord{}, err
	}
	publisher.publishRoot()
	return finished, nil
}

func (o RefreshOrchestrator) executeRootModelTableRun(ctx context.Context, workspaceID string, root RunRecord, tableName string, publish func()) (RunRecord, error) {
	if _, err := o.repo.MarkRunRunning(ctx, workspaceID, root.ID); err != nil {
		return RunRecord{}, err
	}
	if publish != nil {
		publish()
	}
	if err := o.runner.RefreshModelTables(ctx, root.ModelID, []string{tableName}); err != nil {
		failed, finishErr := o.repo.MarkRunFailed(ctx, workspaceID, root.ID, err.Error())
		if finishErr != nil {
			return failed, finishErr
		}
		if publish != nil {
			publish()
		}
		return failed, err
	}
	finished, err := o.repo.MarkRunSucceeded(ctx, workspaceID, root.ID)
	if err != nil {
		return RunRecord{}, err
	}
	if publish != nil {
		publish()
	}
	return finished, nil
}

func (o RefreshOrchestrator) executeSingleModelTableRun(ctx context.Context, workspaceID string, run RunRecord, tableName string, publishTarget func(string)) (RunRecord, error) {
	targetID := run.TargetID
	if publishTarget != nil {
		publishTarget(targetID)
	}
	if _, err := o.repo.MarkRunRunning(ctx, workspaceID, run.ID); err != nil {
		return RunRecord{}, err
	}
	if publishTarget != nil {
		publishTarget(targetID)
	}
	if err := o.runner.RefreshModelTables(ctx, run.ModelID, []string{tableName}); err != nil {
		failed, finishErr := o.repo.MarkRunFailed(ctx, workspaceID, run.ID, err.Error())
		if finishErr != nil {
			return failed, finishErr
		}
		if publishTarget != nil {
			publishTarget(targetID)
		}
		return failed, err
	}
	finished, err := o.repo.MarkRunSucceeded(ctx, workspaceID, run.ID)
	if err != nil {
		return RunRecord{}, err
	}
	if publishTarget != nil {
		publishTarget(targetID)
	}
	return finished, nil
}

func (o RefreshOrchestrator) refreshChildTable(ctx context.Context, workspaceID, modelID, tableName, triggerType, parentRunID, principalID string, publisher RefreshPublisher) error {
	targetID := modelID + "." + tableName
	run, err := o.repo.CreateRun(ctx, RunInput{
		WorkspaceID: workspaceID,
		ModelID:     modelID,
		PrincipalID: principalID,
		TargetType:  TargetModelTable,
		TargetID:    targetID,
		TriggerType: triggerType,
		ParentRunID: parentRunID,
	})
	if err != nil {
		return err
	}
	publisher.publishTarget(targetID)
	if _, err := o.repo.MarkRunRunning(ctx, workspaceID, run.ID); err != nil {
		return err
	}
	publisher.publishTarget(targetID)
	if err := o.runner.RefreshModelTables(ctx, modelID, []string{tableName}); err != nil {
		if _, finishErr := o.repo.MarkRunFailed(ctx, workspaceID, run.ID, err.Error()); finishErr != nil {
			return finishErr
		}
		publisher.publishTarget(targetID)
		return err
	}
	if _, err := o.repo.MarkRunSucceeded(ctx, workspaceID, run.ID); err != nil {
		return err
	}
	publisher.publishTarget(targetID)
	return nil
}

func (o RefreshOrchestrator) failRun(ctx context.Context, workspaceID, runID string, err error, publish func()) error {
	if _, finishErr := o.repo.MarkRunFailed(ctx, workspaceID, runID, err.Error()); finishErr != nil {
		return finishErr
	}
	if publish != nil {
		publish()
	}
	return err
}

func (o RefreshOrchestrator) publishRunFailure(run RunRecord, publisher RefreshPublisher) {
	if run.TargetType == TargetModelTable {
		publisher.publishTarget(run.TargetID)
		return
	}
	publisher.publishRoot()
}

func tableNameFromTargetID(modelID, targetID string) (string, error) {
	prefix := strings.TrimSpace(modelID) + "."
	targetID = strings.TrimSpace(targetID)
	if !strings.HasPrefix(targetID, prefix) {
		return "", fmt.Errorf("model table target %q does not belong to semantic model %q", targetID, modelID)
	}
	tableName := strings.TrimSpace(strings.TrimPrefix(targetID, prefix))
	if tableName == "" {
		return "", fmt.Errorf("model table target id is missing a table name")
	}
	return tableName, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
