package runtime

import "testing"

func TestQueryRuntimeUsesOneVisualizationDataService(t *testing.T) {
	visualizations := &VisualizationDataService{}
	snapshots := &SnapshotService{visualizations: visualizations}
	queries := &QueryService{snapshots: snapshots, visualizations: visualizations}

	if queries.visualizations != snapshots.visualizations {
		t.Fatal("query and snapshot paths must share one visualization data service")
	}
}
