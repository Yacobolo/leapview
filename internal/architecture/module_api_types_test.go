package architecture

import (
	"go/types"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestCapabilityModuleAPIsExposeOnlyContracts(t *testing.T) {
	modulePatterns := []string{}
	seen := map[string]struct{}{}
	for _, file := range productionGoFiles(t) {
		rule, ok := ClassifyPackage(file.pkgDir)
		if !ok || rule.Layer != LayerModule {
			continue
		}
		pattern := modulePath + "/" + file.pkgDir
		if _, ok := seen[pattern]; ok {
			continue
		}
		seen[pattern] = struct{}{}
		modulePatterns = append(modulePatterns, pattern)
	}
	sort.Strings(modulePatterns)
	loaded, err := packages.Load(&packages.Config{
		Dir:  repoRoot(t),
		Mode: packages.NeedName | packages.NeedTypes,
	}, modulePatterns...)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range loaded {
		for _, packageError := range pkg.Errors {
			t.Errorf("load %s: %s", pkg.PkgPath, packageError)
		}
		checkModuleConfigTypes(t, pkg)
		checkModuleMethodResults(t, pkg)
	}
}

func checkModuleConfigTypes(t *testing.T, pkg *packages.Package) {
	t.Helper()
	for _, name := range pkg.Types.Scope().Names() {
		if !strings.HasSuffix(name, "Config") {
			continue
		}
		object := pkg.Types.Scope().Lookup(name)
		named, ok := object.Type().(*types.Named)
		if !ok {
			continue
		}
		structure, ok := named.Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for index := 0; index < structure.NumFields(); index++ {
			field := structure.Field(index)
			if reason := forbiddenModuleConfigType(pkg.PkgPath, field.Type()); reason != "" {
				t.Errorf("%s.%s.%s exposes %s (%s)", pkg.PkgPath, name, field.Name(), types.TypeString(field.Type(), packageQualifier), reason)
			}
		}
	}
}

func checkModuleMethodResults(t *testing.T, pkg *packages.Package) {
	t.Helper()
	object := pkg.Types.Scope().Lookup("Module")
	if object == nil {
		return
	}
	methods := types.NewMethodSet(types.NewPointer(object.Type()))
	for index := 0; index < methods.Len(); index++ {
		method := methods.At(index).Obj()
		if !method.Exported() {
			continue
		}
		signature, ok := method.Type().(*types.Signature)
		if !ok {
			continue
		}
		for result := 0; result < signature.Results().Len(); result++ {
			resultType := signature.Results().At(result).Type()
			if reason := forbiddenModuleResultType(pkg.PkgPath, resultType); reason != "" {
				t.Errorf("%s.(*Module).%s returns %s (%s)", pkg.PkgPath, method.Name(), types.TypeString(resultType, packageQualifier), reason)
			}
		}
	}
}

func forbiddenModuleConfigType(modulePackage string, typ types.Type) string {
	for typ != nil {
		switch value := typ.(type) {
		case *types.Alias:
			typ = types.Unalias(value)
			continue
		case *types.Pointer:
			typ = value.Elem()
			continue
		case *types.Named:
			name := value.Obj().Name()
			path := ""
			if value.Obj().Pkg() != nil {
				path = value.Obj().Pkg().Path()
			}
			if (strings.Contains(name, "Repository") || name == "Persistence") && path != modulePackage {
				return "repositories and persistence are module-owned"
			}
			if strings.Contains(path, "/http") || strings.Contains(path, "/sqlite") ||
				strings.Contains(path, "/control") || strings.Contains(path, "/s3multipart") {
				return "transport and persistence adapters cannot enter module configuration"
			}
			return ""
		case *types.Interface:
			if value.NumMethods() == 0 && value.NumEmbeddeds() == 0 {
				return "opaque any values are not capability contracts"
			}
			for index := 0; index < value.NumEmbeddeds(); index++ {
				if reason := forbiddenModuleConfigType(modulePackage, value.EmbeddedType(index)); reason != "" {
					return reason
				}
			}
			return ""
		default:
			return ""
		}
	}
	return ""
}

func forbiddenModuleResultType(modulePackage string, typ types.Type) string {
	pointer, ok := typ.(*types.Pointer)
	if !ok {
		return ""
	}
	named, ok := pointer.Elem().(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return ""
	}
	path := named.Obj().Pkg().Path()
	if path == modulePackage || strings.Contains(path, "/http") {
		return ""
	}
	for _, adapterPath := range []string{"/ducklake", "/resultcache", "/binding", "/resolver", "/control", "/sqlite", "/filesystem"} {
		if strings.Contains(path, adapterPath) {
			return "return a named contract instead of a concrete adapter"
		}
	}
	return ""
}

func packageQualifier(pkg *types.Package) string {
	if pkg == nil {
		return ""
	}
	return pkg.Path()
}
