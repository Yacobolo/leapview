// Package visualdocs defines the generated contract shared by the visual
// documentation generator and the static documentation site.
package visualdocs

import "github.com/Yacobolo/libredash/internal/dashboard"

const ArtifactVersion = 2

type Artifact struct {
	Version    int                           `json:"version"`
	Documents  map[string][]dashboard.Visual `json:"documents"`
	References map[string]DocumentReference  `json:"references"`
	Showcase   []dashboard.Visual            `json:"showcase"`
}

type DocumentReference struct {
	Kind          string                      `json:"kind"`
	Renderer      string                      `json:"renderer"`
	Shapes        []string                    `json:"shapes"`
	QueryFields   []string                    `json:"queryFields"`
	Options       []string                    `json:"options"`
	Accessibility string                      `json:"accessibility"`
	Examples      map[string]ExampleReference `json:"examples"`
}

type ExampleReference struct {
	KeyFields []string `json:"keyFields"`
}
