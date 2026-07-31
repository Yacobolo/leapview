package mapasset

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sync"
)

// embeddedPackage is the complete worldwide basemap distributed with every
// LeapView binary. Keeping the archive, style, glyphs, and sprites in one
// immutable filesystem makes the map independent of deployment-time files,
// network access, and environment configuration.
//
//go:embed package
var embeddedPackage embed.FS

var (
	embeddedFSOnce     sync.Once
	embeddedFS         fs.FS
	embeddedFSErr      error
	embeddedVerifyOnce sync.Once
	embeddedVerifyErr  error
)

// EmbeddedFS returns the verified-at-build package rooted at
// leapview-streets/. The returned filesystem is immutable and safe to share
// between readiness checks and HTTP handlers.
func EmbeddedFS() fs.FS {
	embeddedFSOnce.Do(func() {
		embeddedFS, embeddedFSErr = fs.Sub(embeddedPackage, "package")
	})
	if embeddedFSErr != nil {
		panic(fmt.Sprintf("open embedded map package: %v", embeddedFSErr))
	}
	return embeddedFS
}

// VerifyEmbedded proves that every embedded file matches the inventory and
// archive size compiled into LeapView. This is a terminal build integrity
// check, not a mutable deployment health check.
func VerifyEmbedded(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	embeddedVerifyOnce.Do(func() {
		embeddedVerifyErr = verifyEmbedded(context.Background())
	})
	return embeddedVerifyErr
}

func verifyEmbedded(ctx context.Context) error {
	return verifyPackageFS(ctx, EmbeddedFS())
}
