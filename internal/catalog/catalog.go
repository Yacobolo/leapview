// Package catalog defines the neutral read model shared by workspace,
// dashboard, agent, admin, and UI transports.
package catalog

type Catalog struct {
	Workspace  Workspace   `json:"workspace"`
	Models     []Model     `json:"models"`
	Dashboards []Dashboard `json:"dashboards"`
}

type Workspace struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Model struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Dashboard struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	SemanticModel string   `json:"semanticModel"`
	Tags          []string `json:"tags"`
	PageCount     int      `json:"pageCount"`
}
