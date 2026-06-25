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
	Items []DashboardSummary `json:"items"`
	Page  PageInfo           `json:"page"`
}

type SemanticModelSummary struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type SemanticModelListResponse struct {
	Items []SemanticModelSummary `json:"items"`
	Page  PageInfo               `json:"page"`
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

type DashboardComponentPlacement struct {
	Col     int `json:"col,omitempty"`
	Row     int `json:"row,omitempty"`
	ColSpan int `json:"colSpan,omitempty"`
	RowSpan int `json:"rowSpan,omitempty"`
}

type DashboardComponentResponse struct {
	ID          string                       `json:"id"`
	Kind        string                       `json:"kind"`
	Ref         string                       `json:"ref,omitempty"`
	Title       string                       `json:"title,omitempty"`
	Description string                       `json:"description,omitempty"`
	Placement   *DashboardComponentPlacement `json:"placement,omitempty"`
	X           float64                      `json:"x,omitempty"`
	Y           float64                      `json:"y,omitempty"`
	Width       float64                      `json:"width,omitempty"`
	Height      float64                      `json:"height,omitempty"`
}

type DashboardComponentListResponse struct {
	Items []DashboardComponentResponse `json:"items"`
	Page  PageInfo                     `json:"page"`
}

type DashboardVisualDescribeResponse struct {
	ID              string                       `json:"id"`
	ComponentID     string                       `json:"componentId,omitempty"`
	Kind            string                       `json:"kind,omitempty"`
	Shape           string                       `json:"shape,omitempty"`
	Renderer        string                       `json:"renderer,omitempty"`
	Type            string                       `json:"type,omitempty"`
	Title           string                       `json:"title,omitempty"`
	Description     string                       `json:"description,omitempty"`
	Query           map[string]any               `json:"query,omitempty"`
	Options         map[string]any               `json:"options,omitempty"`
	RendererOptions map[string]any               `json:"rendererOptions,omitempty"`
	Interaction     map[string]any               `json:"interaction,omitempty"`
	Placement       *DashboardComponentPlacement `json:"placement,omitempty"`
	X               float64                      `json:"x,omitempty"`
	Y               float64                      `json:"y,omitempty"`
	Width           float64                      `json:"width,omitempty"`
	Height          float64                      `json:"height,omitempty"`
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

type DashboardTableDataRequest struct {
	Count   int            `json:"count"`
	Filters map[string]any `json:"filters"`
}

type DashboardFilterOptionResponse struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type DashboardFilterOptionListResponse struct {
	Items []DashboardFilterOptionResponse `json:"items"`
	Page  PageInfo                        `json:"page"`
}
