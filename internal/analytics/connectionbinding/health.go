package connectionbinding

import "time"

type BindingHealthStatus struct {
	BindingID         string              `json:"bindingId"`
	TargetID          string              `json:"targetId"`
	LogicalConnection LogicalConnectionID `json:"logicalConnection"`
	ConnectorKind     string              `json:"connectorKind"`
	Scope             BindingScope        `json:"scope"`
	BindingRevision   int64               `json:"bindingRevision"`
	ValidatedVersion  string              `json:"validatedVersion,omitempty"`
	Health            BindingHealth       `json:"health"`
	Reason            string              `json:"reason,omitempty"`
	LastAttemptAt     time.Time           `json:"lastAttemptAt,omitempty"`
	LastValidatedAt   time.Time           `json:"lastValidatedAt,omitempty"`
	StaleAgeSeconds   int64               `json:"staleAgeSeconds,omitempty"`
	HasActivePool     bool                `json:"hasActivePool"`
}
