package filesystem

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	analyticsduckdb "github.com/Yacobolo/libredash/internal/analytics/duckdb"
	analyticsmaterialize "github.com/Yacobolo/libredash/internal/analytics/materialize"
	"github.com/Yacobolo/libredash/internal/deployment"
	"github.com/Yacobolo/libredash/internal/workspace"
	workspacecompiler "github.com/Yacobolo/libredash/internal/workspace/compiler"
)

type Manifest struct {
	Version        int            `json:"version"`
	WorkspaceID    string         `json:"workspaceId"`
	WorkspaceTitle string         `json:"workspaceTitle"`
	CatalogPath    string         `json:"catalogPath"`
	Files          []ManifestFile `json:"files"`
	SemanticModels []string       `json:"semanticModels"`
	Dashboards     []string       `json:"dashboards"`
}

type ManifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type ValidateOptions struct {
	DataDir   string
	DuckDBDir string
}

func PackProject(projectPath, workspaceID string, out io.Writer) (Manifest, string, error) {
	projectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return Manifest{}, "", err
	}
	if workspaceID == "" {
		return Manifest{}, "", fmt.Errorf("project deploy requires explicit workspace")
	}
	compiled, err := workspacecompiler.CompileProject(projectPath, workspacecompiler.Options{})
	if err != nil {
		return Manifest{}, "", err
	}
	compiledWorkspace, ok := compiled.Workspaces[workspaceID]
	if !ok {
		return Manifest{}, "", fmt.Errorf("project %q has no workspace %q", projectPath, workspaceID)
	}
	baseDir := filepath.Dir(projectPath)
	relFiles, err := collectProjectBundleFiles(baseDir, projectPath)
	if err != nil {
		return Manifest{}, "", err
	}
	manifest := Manifest{
		Version:        1,
		WorkspaceID:    workspaceID,
		WorkspaceTitle: compiledWorkspace.Workspace.Title,
		CatalogPath:    ProjectFile,
		Files:          make([]ManifestFile, 0, len(relFiles)),
	}
	for _, model := range compiledWorkspace.Definition.Catalog.SemanticModels {
		manifest.SemanticModels = append(manifest.SemanticModels, model.ID)
	}
	for _, report := range compiledWorkspace.Definition.Catalog.Dashboards {
		manifest.Dashboards = append(manifest.Dashboards, report.ID)
	}
	return writeBundle(baseDir, relFiles, ProjectFile, projectPath, manifest, out)
}

func collectProjectBundleFiles(baseDir, projectPath string) ([]string, error) {
	relProject, err := filepath.Rel(baseDir, projectPath)
	if err != nil {
		return nil, err
	}
	relFiles := []string{cleanBundlePath(relProject)}
	for _, root := range []string{"connections", "sources", "workspaces"} {
		dir := filepath.Join(baseDir, root)
		if _, err := os.Stat(dir); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml" {
				return nil
			}
			rel, err := filepath.Rel(baseDir, path)
			if err != nil {
				return err
			}
			relFiles = append(relFiles, cleanBundlePath(rel))
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(relFiles[1:])
	return relFiles, nil
}

func writeBundle(baseDir string, relFiles []string, rootRel string, rootPath string, manifest Manifest, out io.Writer) (Manifest, string, error) {
	hash := sha256.New()
	mw := io.MultiWriter(out, hash)
	gz := gzip.NewWriter(mw)
	tw := tar.NewWriter(gz)
	seen := map[string]struct{}{}
	for _, rel := range relFiles {
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		sourcePath := filepath.Join(baseDir, rel)
		if rel == rootRel {
			sourcePath = rootPath
		}
		info, err := os.Stat(sourcePath)
		if err != nil {
			return Manifest{}, "", err
		}
		if info.IsDir() {
			return Manifest{}, "", fmt.Errorf("bundle path %s is a directory", rel)
		}
		bytes, err := os.ReadFile(sourcePath)
		if err != nil {
			return Manifest{}, "", err
		}
		fileHash := sha256.Sum256(bytes)
		manifest.Files = append(manifest.Files, ManifestFile{
			Path:   rel,
			SHA256: hex.EncodeToString(fileHash[:]),
			Size:   info.Size(),
		})
		if err := tw.WriteHeader(&tar.Header{Name: rel, Mode: 0o644, Size: int64(len(bytes))}); err != nil {
			return Manifest{}, "", err
		}
		if _, err := tw.Write(bytes); err != nil {
			return Manifest{}, "", err
		}
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, "", err
	}
	if err := tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0o644, Size: int64(len(manifestBytes))}); err != nil {
		return Manifest{}, "", err
	}
	if _, err := tw.Write(manifestBytes); err != nil {
		return Manifest{}, "", err
	}
	if err := tw.Close(); err != nil {
		return Manifest{}, "", err
	}
	if err := gz.Close(); err != nil {
		return Manifest{}, "", err
	}
	return manifest, hex.EncodeToString(hash.Sum(nil)), nil
}

func ValidateArtifact(path string, workspaceID deployment.WorkspaceID, deploymentID deployment.ID) (deployment.Validation, error) {
	return ValidateArtifactWithOptions(path, workspaceID, deploymentID, ValidateOptions{})
}

func ValidateArtifactWithOptions(path string, workspaceID deployment.WorkspaceID, deploymentID deployment.ID, options ValidateOptions) (deployment.Validation, error) {
	digest, err := fileDigest(path)
	if err != nil {
		return deployment.Validation{}, err
	}
	root, err := os.MkdirTemp("", "libredash-deploy-*")
	if err != nil {
		return deployment.Validation{}, err
	}
	if err := ExtractArtifact(path, root); err != nil {
		os.RemoveAll(root)
		return deployment.Validation{}, err
	}
	manifest, err := readManifest(root)
	if err != nil {
		os.RemoveAll(root)
		return deployment.Validation{}, err
	}
	catalogRel, err := validateManifestFiles(root, manifest)
	if err != nil {
		os.RemoveAll(root)
		return deployment.Validation{}, err
	}
	if workspaceID == "" {
		if strings.TrimSpace(manifest.WorkspaceID) == "" {
			os.RemoveAll(root)
			return deployment.Validation{}, fmt.Errorf("project artifact manifest requires workspaceId")
		}
		workspaceID = deployment.WorkspaceID(manifest.WorkspaceID)
	}
	projectPath := filepath.Join(root, catalogRel)
	project, err := workspacecompiler.CompileProject(projectPath, workspacecompiler.Options{DeploymentID: workspace.DeploymentID(deploymentID)})
	if err != nil {
		os.RemoveAll(root)
		return deployment.Validation{}, err
	}
	compiled, ok := project.Workspaces[string(workspaceID)]
	if !ok {
		os.RemoveAll(root)
		return deployment.Validation{}, fmt.Errorf("project artifact has no workspace %q", workspaceID)
	}
	if options.DataDir != "" {
		if err := discoverSchemasForDefinition(context.Background(), compiled.Definition, options); err != nil {
			os.RemoveAll(root)
			return deployment.Validation{}, err
		}
		graph, err := workspacecompiler.ExtractLineage(workspace.WorkspaceID(workspaceID), workspace.DeploymentID(deploymentID), compiled.Definition)
		if err != nil {
			os.RemoveAll(root)
			return deployment.Validation{}, err
		}
		compiled.Workspace.Graph = graph
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		os.RemoveAll(root)
		return deployment.Validation{}, err
	}
	return deployment.Validation{
		Digest:       digest,
		ManifestJSON: string(manifestJSON),
		RootDir:      root,
		Graph:        compiled.Workspace.Graph,
	}, nil
}

func discoverSchemasForDefinition(ctx context.Context, definition *workspace.Definition, options ValidateOptions) error {
	duckDBRoot := options.DuckDBDir
	removeDuckDBRoot := false
	if duckDBRoot == "" {
		var err error
		duckDBRoot, err = os.MkdirTemp("", "libredash-schema-*")
		if err != nil {
			return err
		}
		removeDuckDBRoot = true
	}
	if removeDuckDBRoot {
		defer os.RemoveAll(duckDBRoot)
	}
	for _, entry := range definition.Catalog.SemanticModels {
		model := definition.Models[entry.ID]
		dbDir := filepath.Join(duckDBRoot, entry.ID)
		dbPath := analyticsmaterialize.DatabasePath(dbDir, entry.ID)
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return err
		}
		db, err := analyticsduckdb.Open(ctx, dbPath)
		if err != nil {
			return err
		}
		sources := analyticsduckdb.NewSourceRuntime(db, options.DataDir)
		if _, err := analyticsmaterialize.Refresh(ctx, db, sources, model); err != nil {
			db.Close()
			return err
		}
		if err := analyticsduckdb.DiscoverSchemas(ctx, db, model); err != nil {
			db.Close()
			return err
		}
		if err := db.Close(); err != nil {
			return err
		}
	}
	return nil
}

func ExtractArtifact(path, dest string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		rel, err := safeBundlePath(header.Name)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) && filepath.Clean(target) != filepath.Clean(dest) {
			return fmt.Errorf("bundle path %q escapes destination", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported bundle entry %q", header.Name)
		}
	}
}

func readManifest(root string) (Manifest, error) {
	bytes, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(bytes, &manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.CatalogPath == "" {
		manifest.CatalogPath = ProjectFile
	}
	return manifest, nil
}

func validateManifestFiles(root string, manifest Manifest) (string, error) {
	catalogRel, err := safeBundlePath(manifest.CatalogPath)
	if err != nil {
		return "", fmt.Errorf("invalid catalog path: %w", err)
	}
	seen := map[string]struct{}{}
	hasCatalog := false
	for _, file := range manifest.Files {
		rel, err := safeBundlePath(file.Path)
		if err != nil {
			return "", fmt.Errorf("invalid manifest file path %q: %w", file.Path, err)
		}
		if _, ok := seen[rel]; ok {
			return "", fmt.Errorf("duplicate manifest file path %q", rel)
		}
		seen[rel] = struct{}{}
		if rel == catalogRel {
			hasCatalog = true
		}
		path := filepath.Join(root, rel)
		bytes, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(bytes)
		if got := hex.EncodeToString(sum[:]); got != file.SHA256 {
			return "", fmt.Errorf("file %s digest mismatch", file.Path)
		}
	}
	if !hasCatalog {
		return "", fmt.Errorf("catalog path %q is not listed in manifest files", manifest.CatalogPath)
	}
	return catalogRel, nil
}

func fileDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func cleanBundlePath(path string) string {
	path = filepath.ToSlash(filepath.Clean(path))
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimPrefix(path, "../")
	return path
}

func safeBundlePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("bundle path %q must be relative", path)
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || clean == "" {
		return "", fmt.Errorf("bundle path %q is empty", path)
	}
	for _, part := range strings.Split(clean, "/") {
		if part == ".." {
			return "", fmt.Errorf("bundle path %q escapes bundle root", path)
		}
	}
	return clean, nil
}

func workspaceTitle(value string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return "LibreDash Workspace"
}
