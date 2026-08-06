package artifact

import (
	"fmt"
	"sort"
	"strings"

	"github.com/flidai/leapview/internal/analytics/connectors"
)

type ConnectionActivationMode = connectors.ActivationMode

const (
	ManagedActivation       = connectors.ManagedActivation
	AuthoredActivation      = connectors.AuthoredActivation
	TargetBindingActivation = connectors.TargetBindingActivation
)

// ConnectionActivation is the immutable deployment-facing projection of a
// compiled logical connection. The connector registry remains the single
// source of truth for how each kind is activated.
type ConnectionActivation struct {
	LogicalConnectionID string
	ConnectorKind       string
	Mode                ConnectionActivationMode
}

func (w Workspace) ConnectionActivations() ([]ConnectionActivation, error) {
	definition := w.Manifest()
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
	result := make([]ConnectionActivation, 0, len(connectionIDs))
	for _, connectionID := range connectionIDs {
		kind := kinds[connectionID]
		spec, ok := connectors.LookupConnection(kind)
		if !ok {
			return nil, fmt.Errorf(
				"compiled workspace connection %q has unsupported connector kind %q",
				connectionID,
				kind,
			)
		}
		if spec.ActivationMode == "" {
			return nil, fmt.Errorf(
				"compiled workspace connection %q has no activation mode",
				connectionID,
			)
		}
		result = append(result, ConnectionActivation{
			LogicalConnectionID: connectionID,
			ConnectorKind:       kind,
			Mode:                spec.ActivationMode,
		})
	}
	return result, nil
}
