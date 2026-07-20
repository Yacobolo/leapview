// Package mapasset owns the immutable, content-addressed cartographic style
// packages available to compiled visualization specifications.
package mapasset

import (
	"fmt"

	visualizationir "github.com/Yacobolo/libredash/internal/visualization/ir"
)

var assets = map[string]visualizationir.VisualizationMapStyleAsset{
	"streets": {
		ID:            "libredash-streets",
		StyleURL:      "/map-assets/libredash-streets/style.json",
		StyleDigest:   "sha256:c62e1d3f357246401f8237f5416ab0e4421c7463ac4f00f899bcfe6eeace2bab",
		ArchiveURL:    "/map-assets/libredash-streets/basemap.pmtiles",
		ArchiveDigest: "sha256:2d97ee8907670936ab722da7ca06eafec0734392f73fa1cd337d4debd85d676f",
		GlyphsURL:     "/map-assets/libredash-streets/glyphs/{fontstack}/{range}.pbf",
		SpriteURL:     "/map-assets/libredash-streets/sprites/libredash",
		Source:        "OpenStreetMap contributors; packaged as an immutable LibreDash vector basemap",
		License:       "Open Database License 1.0 (data); BSD-3-Clause (style)",
		Attribution:   "© OpenStreetMap contributors",
		MinimumZoom:   0,
		MaximumZoom:   6,
		Bounds:        []float64{-180, -85.051129, 180, 85.051129},
		LabelAnchor:   "address_label",
	},
}

// Resolve returns a complete provenance record for a public authoring asset.
func Resolve(id string) (visualizationir.VisualizationMapStyleAsset, error) {
	asset, ok := assets[id]
	if !ok {
		return visualizationir.VisualizationMapStyleAsset{}, fmt.Errorf("unknown map style asset %q", id)
	}
	return asset, nil
}
