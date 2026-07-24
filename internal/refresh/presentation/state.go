package presentation

import "time"

// AssetRefreshState is the refresh capability's presentation read model for a
// workspace asset. Consumers may adapt it into their own page contracts.
type AssetRefreshState struct {
	CSRFToken        string
	Runs             []AssetRefreshRun
	Latest           AssetRefreshRun
	LatestSuccessful AssetRefreshRun
	DataVersion      AssetDataVersion
	NextRun          time.Time
}

type AssetDataVersion struct {
	SnapshotID     int64
	ServingStateID string
	RefreshedAt    time.Time
	Source         string
}

type AssetRefreshRun struct {
	ID                   string
	PrincipalDisplayName string
	TriggerType          string
	Status               string
	StartedAt            string
	FinishedAt           string
	Error                string
}
