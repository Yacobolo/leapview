package reportmodel

import (
	"fmt"
	"strings"

	semanticmodel "github.com/Yacobolo/libredash/internal/analytics/model"
	dashboarddefinition "github.com/Yacobolo/libredash/internal/dashboard/definition"
	visualizationdefinition "github.com/Yacobolo/libredash/internal/visualization/definition"
	visualizationir "github.com/Yacobolo/libredash/internal/visualization/ir"
)

// ResolveCompiledSelectionInteraction resolves the semantic types of the
// compiler-owned IR mappings without reconstructing authoring dashboard models.
func ResolveCompiledSelectionInteraction(definition *dashboarddefinition.Definition, model *semanticmodel.Model, sourceKind, sourceID string) (ResolvedSelectionInteraction, error) {
	if sourceKind != "visual" {
		return ResolvedSelectionInteraction{}, fmt.Errorf("unknown source kind %q", sourceKind)
	}
	source, ok := definition.Visualizations[sourceID]
	if !ok {
		return ResolvedSelectionInteraction{}, fmt.Errorf("unknown source visualization %q", sourceID)
	}
	base, err := visualizationir.SpecificationBase(source.Spec)
	if err != nil {
		return ResolvedSelectionInteraction{}, err
	}
	if len(base.Interactions) == 0 {
		return ResolvedSelectionInteraction{}, fmt.Errorf("visualization %q has no selection interaction", sourceID)
	}
	interaction := base.Interactions[0]
	resolved := ResolvedSelectionInteraction{Mappings: make([]ResolvedSelectionMapping, 0, len(interaction.Mappings))}
	for index, mapping := range interaction.Mappings {
		item, err := resolveCompiledMapping(model, mapping)
		if err != nil {
			return ResolvedSelectionInteraction{}, fmt.Errorf("visualization %q interaction mapping %d: %w", sourceID, index, err)
		}
		resolved.Mappings = append(resolved.Mappings, item)
	}
	for _, targetID := range interaction.Targets {
		target, ok := definition.Visualizations[targetID]
		if !ok {
			return ResolvedSelectionInteraction{}, fmt.Errorf("interaction references unknown target %q", targetID)
		}
		kind := "visual"
		if target.Query.Kind == visualizationdefinition.QueryDetail || target.Query.Kind == visualizationdefinition.QueryMatrix || target.Query.Kind == visualizationdefinition.QueryPivot {
			kind = "table"
		}
		resolved.Targets = append(resolved.Targets, ResolvedSelectionTarget{Kind: kind, ID: targetID})
	}
	return resolved, nil
}

func resolveCompiledMapping(model *semanticmodel.Model, mapping visualizationir.VisualizationInteractionMapping) (ResolvedSelectionMapping, error) {
	field, fact, grain := mapping.TargetFieldID, "", ""
	if mapping.TargetFactID != nil {
		fact = *mapping.TargetFactID
	}
	if mapping.Grain != nil {
		grain = *mapping.Grain
	}
	if !strings.Contains(field, ".") {
		dimension, err := model.ResolveSemanticDimension(field)
		if err != nil {
			return ResolvedSelectionMapping{}, err
		}
		return ResolvedSelectionMapping{Field: field, Grain: grain, Type: dimension.Type, Scope: SelectionScopeConformed}, nil
	}
	if fact == "" {
		return ResolvedSelectionMapping{}, fmt.Errorf("physical field %q requires fact", field)
	}
	dimension, err := model.ResolveDimension(field)
	if err != nil {
		return ResolvedSelectionMapping{}, err
	}
	if err := model.CanReachField(fact, field); err != nil {
		return ResolvedSelectionMapping{}, err
	}
	return ResolvedSelectionMapping{Field: field, Fact: fact, Grain: grain, Type: dimension.Type, Scope: SelectionScopeFactLocal}, nil
}
