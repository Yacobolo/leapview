package datastar

import (
	"net/http"

	"github.com/Yacobolo/libredash/internal/dashboard"
	"github.com/Yacobolo/libredash/internal/dashboard/command"
	dashboardstream "github.com/Yacobolo/libredash/internal/dashboard/stream"
	"github.com/Yacobolo/libredash/pkg/pagestream"
)

func DashboardID(r *http.Request, signals dashboard.Signals, defaultID string) string {
	if id := r.URL.Query().Get("dashboard"); id != "" {
		return id
	}
	if signals.Runtime.DashboardID != "" {
		return signals.Runtime.DashboardID
	}
	return defaultID
}

func PageID(r *http.Request, signals dashboard.Signals) string {
	if id := r.URL.Query().Get("page"); id != "" {
		return id
	}
	if signals.Runtime.PageID != "" {
		return signals.Runtime.PageID
	}
	return ""
}

func ModelID(r *http.Request, signals dashboard.Signals, dashboardID string, defaultForDashboard func(string) string) string {
	if id := r.URL.Query().Get("model"); id != "" {
		return id
	}
	if signals.Runtime.ModelID != "" {
		return signals.Runtime.ModelID
	}
	return defaultForDashboard(dashboardID)
}

func ClientStreamID(r *http.Request, signals dashboard.Signals, dashboardID, pageID string) string {
	return pagestream.ClientIDFromRequest(r, signals.Runtime.ClientID) + ":" + dashboardID + ":" + pageID
}

func DashboardPatch(patch dashboard.Patch) pagestream.Patch {
	return pagestream.Patch{
		"filters":       patch.Filters,
		"filterOptions": patch.FilterOptions,
		"status":        patch.Status,
		"visuals":       patch.Visuals,
	}
}

func TablePatch(name string, table dashboard.Table) pagestream.Patch {
	return pagestream.Patch{
		"tables": map[string]dashboard.Table{
			name: table,
		},
	}
}

func TablesPatch(tables map[string]dashboard.Table) pagestream.Patch {
	return pagestream.Patch{"tables": tables}
}

func LoadingPatch(dataDir string) pagestream.Patch {
	return pagestream.Patch{
		"status": map[string]any{
			"loading":       true,
			"error":         "",
			"dataDirectory": dataDir,
		},
	}
}

func CommandEventPatch(event command.Event) pagestream.Patch {
	switch event.Type {
	case command.EventLoading:
		return LoadingPatch(event.DataDir)
	case command.EventDashboard:
		return DashboardPatch(event.Patch)
	case command.EventTables:
		return TablesPatch(event.Tables)
	case command.EventTable:
		return TablePatch(event.TableName, event.Table)
	default:
		return pagestream.Patch{}
	}
}

func SnapshotPatches(snapshot dashboardstream.Snapshot) []pagestream.Patch {
	return []pagestream.Patch{
		DashboardPatch(snapshot.Patch),
		TablesPatch(snapshot.Tables),
	}
}
