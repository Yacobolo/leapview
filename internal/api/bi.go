package api

type DashboardSummary struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	SemanticModel string   `json:"semanticModel"`
	Tags          []string `json:"tags"`
	PageCount     int      `json:"pageCount"`
}

type DashboardListResponse struct {
	Dashboards []DashboardSummary `json:"dashboards"`
}

type SemanticModelSummary struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type SemanticModelListResponse struct {
	Models []SemanticModelSummary `json:"models"`
}

type ModelRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type DashboardManifestResponse struct {
	ID            string                  `json:"id"`
	Title         string                  `json:"title"`
	Description   string                  `json:"description,omitempty"`
	SemanticModel string                  `json:"semantic_model,omitempty"`
	Model         *ModelRef               `json:"model,omitempty"`
	Counts        DashboardManifestCounts `json:"counts"`
	Pages         []DashboardManifestPage `json:"pages"`
	DetailTools   map[string]string       `json:"detail_tools"`
}

type DashboardManifestCounts struct {
	Pages   int `json:"pages"`
	Visuals int `json:"visuals"`
	Tables  int `json:"tables"`
	Filters int `json:"filters"`
}

type DashboardManifestPage struct {
	ID          string                       `json:"id"`
	Title       string                       `json:"title"`
	Description string                       `json:"description,omitempty"`
	Components  []DashboardManifestComponent `json:"components"`
}

type DashboardManifestComponent struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Ref   string `json:"ref"`
	Title string `json:"title,omitempty"`
}

type SemanticModelDescriptionResponse struct {
	ID          string                      `json:"id"`
	Title       string                      `json:"title"`
	Description string                      `json:"description"`
	Dashboards  []ModelDashboardUsage       `json:"dashboards"`
	Counts      *SemanticModelCounts        `json:"counts,omitempty"`
	Tables      []SemanticModelTableSummary `json:"tables,omitempty"`
}

type SemanticModelCounts struct {
	Sources       int `json:"sources"`
	ModelTables   int `json:"model_tables"`
	Fields        int `json:"fields"`
	Measures      int `json:"measures"`
	Relationships int `json:"relationships"`
}

type SemanticModelTableSummary struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Source      string `json:"source"`
	Description string `json:"description"`
	Fields      int    `json:"fields"`
}

type ModelDashboardUsage struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	SemanticModel string `json:"semantic_model"`
	Pages         int    `json:"pages"`
}

type DashboardPageQueryRequest struct {
	Filters map[string]any `json:"filters"`
}

type DashboardTableQueryRequest struct {
	PageID  string         `json:"pageId"`
	Count   int            `json:"count"`
	Filters map[string]any `json:"filters"`
}
