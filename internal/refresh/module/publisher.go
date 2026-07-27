package module

import "context"

// Publisher adapts refresh-owned workspace presentation and an optional
// semantic-model notification port to the durable refresh workflow.
type Publisher struct {
	Workspace            func() WorkspaceSupport
	SemanticModelVersion func(context.Context, string, string, string)
}

func (p Publisher) PublishRefreshTarget(ctx context.Context, workspaceID, environment, targetType, targetID string) {
	if p.Workspace != nil {
		p.Workspace().PublishWorkspaceAssetRefreshPatchesForTarget(ctx, workspaceID, environment, targetType, targetID)
	}
}

func (p Publisher) PublishSemanticModelVersion(ctx context.Context, workspaceID, environment, modelID string) {
	if p.SemanticModelVersion != nil {
		p.SemanticModelVersion(ctx, workspaceID, environment, modelID)
	}
}
