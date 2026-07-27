package app

import (
	"path/filepath"

	"github.com/Yacobolo/leapview/internal/app/config"
	"github.com/Yacobolo/leapview/internal/platform/web/staticasset"
)

func applicationAssets(config config.Config, production bool) staticasset.Resolver {
	return staticasset.New(staticasset.Config{
		Production: production,
		Version:    config.AssetVersion,
		GeneratedVersionPath: filepath.Join(
			"static",
			"asset-version.txt",
		),
	})
}
