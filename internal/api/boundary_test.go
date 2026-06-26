package api_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	apigen "github.com/Yacobolo/libredash/internal/api/gen"
)

func TestAPIPackageStaysTransportContractOnly(t *testing.T) {
	forbidden := map[string]bool{
		"github.com/Yacobolo/libredash/internal/app":     true,
		"github.com/Yacobolo/libredash/internal/ui":      true,
		"github.com/go-chi/chi/v5":                       true,
		"github.com/starfederation/datastar-go/datastar": true,
		"maragu.dev/gomponents":                          true,
		"maragu.dev/gomponents-datastar":                 true,
		"net/http":                                       true,
	}
	assertPackageDoesNotImport(t, ".", forbidden)
}

func TestAgentAppDoesNotDependOnHeadlessAPIContract(t *testing.T) {
	assertPackageDoesNotImport(t, filepath.Join("..", "agentapp"), map[string]bool{
		"github.com/Yacobolo/libredash/internal/api": true,
	})
}

func TestGeneratedAssetResponseRequiresSnapshotAndPayload(t *testing.T) {
	typ := reflect.TypeOf(apigen.AssetResponse{})
	for _, name := range []string{"SnapshotId", "Payload"} {
		field, ok := typ.FieldByName(name)
		if !ok {
			t.Fatalf("AssetResponse.%s missing", name)
		}
		if field.Type.Kind() == reflect.Pointer {
			t.Fatalf("AssetResponse.%s is optional pointer type %s", name, field.Type)
		}
		if strings.Contains(string(field.Tag), "omitempty") {
			t.Fatalf("AssetResponse.%s JSON tag is optional: %s", name, field.Tag)
		}
	}
}

func assertPackageDoesNotImport(t *testing.T, dir string, forbidden map[string]bool) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("glob %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		info, err := os.Stat(file)
		if err != nil {
			t.Fatalf("stat %s: %v", file, err)
		}
		if info.IsDir() {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse imports in %s: %v", file, err)
		}
		for _, imported := range parsed.Imports {
			path := strings.Trim(imported.Path.Value, "\"")
			if forbidden[path] {
				t.Fatalf("%s imports forbidden package %s", file, path)
			}
		}
	}
}
