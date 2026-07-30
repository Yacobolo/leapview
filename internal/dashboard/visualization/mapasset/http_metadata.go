package mapasset

import (
	"mime"
	"path/filepath"
	"strings"
)

const ImmutableCacheControl = "public, max-age=31536000, immutable"

// ContentType returns the canonical representation metadata for an embedded
// package path.
func ContentType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pmtiles":
		return "application/vnd.pmtiles"
	case ".pbf":
		return "application/x-protobuf"
	case ".json":
		return "application/json"
	case ".png":
		return "image/png"
	default:
		if value := mime.TypeByExtension(filepath.Ext(path)); value != "" {
			return value
		}
		return "application/octet-stream"
	}
}
