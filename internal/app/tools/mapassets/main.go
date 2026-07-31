package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	visualizationmapasset "github.com/flidai/leapview/internal/dashboard/visualization/mapasset"
)

const (
	planetURL              = visualizationmapasset.PlanetSnapshotURL
	archiveDigest          = visualizationmapasset.ArchiveSHA256
	archiveDownloadThreads = "2"
	basemapAssetsSHA       = visualizationmapasset.BasemapAssetsRevision
	pmtilesToolModule      = "github.com/protomaps/go-pmtiles@" + visualizationmapasset.PMTilesToolVersion
)

var glyphRanges = []string{
	"0-255",
	"256-511",
	"512-767",
	"768-1023",
	"1024-1279",
	"1280-1535",
	"1536-1791",
	"4096-4351",
	"11520-11775",
}

func main() {
	out := flag.String("out", "internal/dashboard/visualization/mapasset/package", "embedded map package output directory")
	seedArchive := flag.String("seed-archive", "", "verified pinned archive to reuse instead of extracting it")
	flag.Parse()
	ctx := context.Background()
	if strings.TrimSpace(*seedArchive) != "" {
		if err := installSeedArchive(*seedArchive, *out); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if err := install(ctx, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func installSeedArchive(source, out string) error {
	if err := verifyFile(source, archiveDigest); err != nil {
		return fmt.Errorf("verify seed map archive: %w", err)
	}
	asset, err := visualizationmapasset.Resolve("streets")
	if err != nil {
		return err
	}
	target, err := assetTarget(out, asset.ArchiveURL)
	if err != nil {
		return err
	}
	temporary := target + ".seed.partial"
	if err := os.Remove(temporary); err != nil && !os.IsNotExist(err) {
		return err
	}
	defer os.Remove(temporary)
	if err := copyFile(source, temporary); err != nil {
		return fmt.Errorf("copy seed map archive: %w", err)
	}
	if err := os.Rename(temporary, target); err != nil {
		return err
	}
	return nil
}

func install(ctx context.Context, out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	asset, err := visualizationmapasset.Resolve("streets")
	if err != nil {
		return err
	}
	archive, err := assetTarget(out, asset.ArchiveURL)
	if err != nil {
		return err
	}
	legacyArchive := filepath.Join(out, "leapview-streets", "basemap.pmtiles")
	if err := ensureArchive(ctx, archive, legacyArchive); err != nil {
		return err
	}
	style, err := assetTarget(out, asset.StyleURL)
	if err != nil {
		return err
	}
	if err := copyFile("static/map-assets/leapview-streets/style.json", style); err != nil {
		return fmt.Errorf("install map style: %w", err)
	}
	if err := verifyFile(style, visualizationmapasset.StyleSHA256); err != nil {
		return err
	}
	client := &http.Client{Timeout: 45 * time.Second}
	for _, font := range []string{"Noto Sans Regular", "Noto Sans Medium", "Noto Sans Italic"} {
		for _, glyphRange := range glyphRanges {
			assetURL := strings.ReplaceAll(strings.ReplaceAll(asset.GlyphsURL, "{fontstack}", url.PathEscape(font)), "{range}", glyphRange)
			target, err := assetTarget(out, assetURL)
			if err != nil {
				return err
			}
			expected, err := expectedDigest(assetURL)
			if err != nil {
				return err
			}
			remote := fmt.Sprintf("https://raw.githubusercontent.com/protomaps/basemaps-assets/%s/fonts/%s/%s.pbf", basemapAssetsSHA, url.PathEscape(font), glyphRange)
			if err := downloadIfMissing(ctx, client, remote, target, expected); err != nil {
				return err
			}
		}
	}
	for _, suffix := range []string{".json", ".png", "@2x.json", "@2x.png"} {
		assetURL := asset.SpriteURL + suffix
		target, err := assetTarget(out, assetURL)
		if err != nil {
			return err
		}
		expected, err := expectedDigest(assetURL)
		if err != nil {
			return err
		}
		remote := fmt.Sprintf("https://raw.githubusercontent.com/protomaps/basemaps-assets/%s/sprites/v4/light%s", basemapAssetsSHA, suffix)
		if err := downloadIfMissing(ctx, client, remote, target, expected); err != nil {
			return err
		}
	}
	return visualizationmapasset.VerifyGeneratedPackage(out)
}

func ensureArchive(ctx context.Context, target, legacy string) error {
	if _, err := os.Stat(target); err == nil {
		return verifyFile(target, archiveDigest)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	build, err := os.MkdirTemp(filepath.Dir(target), ".map-build-")
	if err != nil {
		return fmt.Errorf("create map archive build directory: %w", err)
	}
	defer os.RemoveAll(build)
	global := filepath.Join(build, "worldwide-z0-z6.pmtiles")
	if err := reuseVerifiedArchive(target, legacy, archiveDigest, global); err != nil {
		if err := runPMTiles(ctx, "extract", planetURL, global, "--maxzoom=6", "--download-threads="+archiveDownloadThreads); err != nil {
			return fmt.Errorf("extract pinned global PMTiles: %w", err)
		}
		if err := verifyFile(global, archiveDigest); err != nil {
			return err
		}
	}
	temporary := target + ".partial"
	if err := os.Remove(temporary); err != nil && !os.IsNotExist(err) {
		return err
	}
	defer os.Remove(temporary)
	if err := copyFile(global, temporary); err != nil {
		return fmt.Errorf("stage worldwide PMTiles: %w", err)
	}
	if err := verifyFile(temporary, archiveDigest); err != nil {
		return err
	}
	if err := os.Rename(temporary, target); err != nil {
		return err
	}
	return nil
}

func reuseVerifiedArchive(primary, legacy, digest, target string) error {
	for _, candidate := range []string{primary, legacy} {
		if candidate == "" {
			continue
		}
		if err := verifyFile(candidate, digest); err == nil {
			if err := copyFile(candidate, target); err != nil {
				return fmt.Errorf("reuse verified map archive: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("verified map archive %s is not installed", digest)
}

func runPMTiles(ctx context.Context, arguments ...string) error {
	command := exec.CommandContext(ctx, "go", append([]string{"run", pmtilesToolModule}, arguments...)...)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	return command.Run()
}

func downloadIfMissing(ctx context.Context, client *http.Client, remote, target, expected string) error {
	if info, err := os.Stat(target); err == nil && info.Size() > 0 {
		if err := verifyFile(target, expected); err == nil {
			return nil
		}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, remote, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("download %s: %w", remote, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", remote, response.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	temporary := target + ".partial"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := verifyFile(temporary, expected); err != nil {
		return err
	}
	return os.Rename(temporary, target)
}

func assetTarget(root, value string) (string, error) {
	if !visualizationmapasset.IsContentAddressedURLPath(value) {
		return "", fmt.Errorf("map asset URL is not content addressed: %q", value)
	}
	decoded, err := url.PathUnescape(strings.TrimPrefix(value, "/map-assets/"))
	if err != nil {
		return "", err
	}
	target := filepath.Join(root, filepath.FromSlash(decoded))
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("map asset target escapes root")
	}
	return target, nil
}

func expectedDigest(value string) (string, error) {
	decoded, err := url.PathUnescape(strings.TrimPrefix(value, "/map-assets/"))
	if err != nil {
		return "", err
	}
	for _, file := range visualizationmapasset.ExpectedFiles() {
		if file.Path == decoded {
			return file.Digest, nil
		}
	}
	return "", fmt.Errorf("map asset %q is not in the compiled inventory", value)
}

func verifyFile(name, expected string) error {
	file, err := os.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := fmt.Sprintf("%x", hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("map asset %s digest mismatch: got %s", name, actual)
	}
	return nil
}

func copyFile(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
