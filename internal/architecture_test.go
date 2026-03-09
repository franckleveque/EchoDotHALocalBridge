package internal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kcmvp/archunit"
)

// ============================================================
// LAYER DEFINITIONS (shared by all tests)
// ============================================================

var (
	domainLayer   = archunit.Packages("domain", []string{".../internal/domain/..."})
	adaptersLayer = archunit.Packages("adapters", []string{".../internal/adapters/..."})
	portsLayer    = archunit.Packages("ports", []string{".../internal/ports/..."})

	inputAdapters  = archunit.Packages("input-adapters", []string{".../internal/adapters/input/..."})
	outputAdapters = archunit.Packages("output-adapters", []string{".../internal/adapters/output/..."})

	serviceLayer    = archunit.Packages("service", []string{".../internal/domain/service/..."})
	translatorLayer = archunit.Packages("translator", []string{".../internal/domain/translator/..."})
	modelLayer      = archunit.Packages("model", []string{".../internal/domain/model/..."})
)

// ============================================================
// EXISTING TEST (kept for reference)
// ============================================================

func TestArchitecture(t *testing.T) {
	// Rule 1: Domain should not depend on adapters
	if err := domainLayer.ShouldNotReferLayers(adaptersLayer); err != nil {
		t.Errorf("Architecture violation: Domain depends on Adapters: %v", err)
	}
}

// ============================================================
// NEW: HEXAGONAL BOUNDARY TESTS
// ============================================================

// TestPortsShouldNotDependOnAdapters ensures that the ports package,
// which defines the contracts (interfaces) of the hexagon, never imports
// concrete adapter implementations. Ports must remain infrastructure-agnostic.
func TestPortsShouldNotDependOnAdapters(t *testing.T) {
	if err := portsLayer.ShouldNotReferLayers(adaptersLayer); err != nil {
		t.Errorf("Hexagonal violation: Ports depend on Adapters — ports must be pure interfaces: %v", err)
	}
}

// TestAdaptersShouldNotImportConcreteServices checks the most critical violation
// found in the codebase: the HTTP input adapter imports domain/service (a concrete
// package) instead of depending only on ports interfaces.
// Adapters MUST depend on ports, never on concrete service implementations.
func TestAdaptersShouldNotImportConcreteServices(t *testing.T) {
	if err := inputAdapters.ShouldNotReferLayers(serviceLayer); err != nil {
		t.Errorf(`Hexagonal violation: Input adapter depends on concrete domain/service package.
Fix: define an AuthPort input interface in ports/ and inject it instead of *service.AuthService.
Details: %v`, err)
	}
}

// TestAdaptersShouldNotInstantiateDomainObjects catches the pattern where
// the HTTP adapter creates a translator.Factory internally.
// Input adapters should not construct domain objects; that is the composition root's job.
func TestInputAdaptersShouldNotImportTranslators(t *testing.T) {
	if err := inputAdapters.ShouldNotReferLayers(translatorLayer); err != nil {
		t.Errorf(`Hexagonal violation: Input adapter imports domain/translator.
Fix: expose HueMetadata via BridgePort (e.g. GetDeviceMetadata) so the adapter does not
need to know about translators. Details: %v`, err)
	}
}

// TestOutputAdaptersShouldNotImportInputAdapters enforces that the output side
// of the hexagon never depends on the input side.
func TestOutputAdaptersShouldNotImportInputAdapters(t *testing.T) {
	if err := outputAdapters.ShouldNotReferLayers(inputAdapters); err != nil {
		t.Errorf("Hexagonal violation: Output adapter depends on Input adapter: %v", err)
	}
}

// TestInputAdaptersShouldNotImportOutputAdapters enforces the symmetric rule:
// HTTP/SSDP adapters must not directly call the HA client or persistence layer.
func TestInputAdaptersShouldNotImportOutputAdapters(t *testing.T) {
	if err := inputAdapters.ShouldNotReferLayers(outputAdapters); err != nil {
		t.Errorf("Hexagonal violation: Input adapter depends on Output adapter (bypasses ports): %v", err)
	}
}

// TestModelShouldNotDependOnPorts ensures the domain model stays a pure data layer,
// free of any port/interface concerns.
func TestModelShouldNotDependOnPorts(t *testing.T) {
	if err := modelLayer.ShouldNotReferLayers(portsLayer); err != nil {
		t.Errorf("Architecture violation: Domain model imports ports — model must be dependency-free: %v", err)
	}
}

// ============================================================
// NEW: SOLID / DESIGN RULE TESTS (AST-based)
// These rules require source inspection beyond what archunit's
// layer API provides. They use Go's go/parser directly.
// ============================================================

// projectRoot returns the absolute path to the repository root.
// It walks up from the test file location until it finds go.mod.
func projectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cannot get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod in any parent directory")
		}
		dir = parent
	}
}

// collectGoFiles returns all .go files (excluding test files) under root.
func collectGoFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// importsOf returns the set of import paths in a given Go source file.
func importsOf(path string) (map[string]bool, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	imports := make(map[string]bool)
	for _, imp := range f.Imports {
		// Strip surrounding quotes
		importPath := strings.Trim(imp.Path.Value, `"`)
		imports[importPath] = true
	}
	return imports, nil
}

// TestConcreteTypesNotInjectedAcrossBoundaries detects fields in adapter structs
// that are typed as concrete domain service structs (pointer to struct from domain/service).
// This catches the *service.AuthService injection in the HTTP adapter.
func TestConcreteTypesNotInjectedAcrossBoundaries(t *testing.T) {
	root := projectRoot(t)
	adapterDir := filepath.Join(root, "internal", "adapters")
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, filepath.Join(adapterDir, "input", "http"), nil, 0)
	if err != nil {
		t.Fatalf("cannot parse http adapter: %v", err)
	}

	violations := []string{}
	for _, pkg := range pkgs {
		for filename, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				structType, ok := n.(*ast.StructType)
				if !ok {
					return true
				}
				for _, field := range structType.Fields.List {
					// Check for *pkg.Type where pkg is a domain/service package
					starExpr, ok := field.Type.(*ast.StarExpr)
					if !ok {
						continue
					}
					selectorExpr, ok := starExpr.X.(*ast.SelectorExpr)
					if !ok {
						continue
					}
					pkgIdent, ok := selectorExpr.X.(*ast.Ident)
					if !ok {
						continue
					}
					// Flag any field whose type comes from the "service" package
					if pkgIdent.Name == "service" {
						fieldNames := []string{}
						for _, name := range field.Names {
							fieldNames = append(fieldNames, name.Name)
						}
						violations = append(violations, filepath.Base(filename)+
							": field(s) "+strings.Join(fieldNames, ", ")+
							" typed as *service."+selectorExpr.Sel.Name+
							" — should be an interface from ports/")
					}
				}
				return true
			})
		}
	}

	if len(violations) > 0 {
		t.Errorf("DIP violation: adapter struct fields use concrete service types instead of port interfaces:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// TestTranslatorFactoryNotCreatedInAdapters ensures that no adapter file
// calls translator.NewFactory(), which would mean the adapter is constructing
// domain objects rather than receiving them via dependency injection.
func TestTranslatorFactoryNotCreatedInAdapters(t *testing.T) {
	root := projectRoot(t)
	adapterDir := filepath.Join(root, "internal", "adapters")

	files, err := collectGoFiles(adapterDir)
	if err != nil {
		t.Fatalf("cannot walk adapter directory: %v", err)
	}

	violations := []string{}
	for _, path := range files {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("cannot parse %s: %v", path, err)
		}

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if pkg.Name == "translator" && sel.Sel.Name == "NewFactory" {
				pos := fset.Position(call.Pos())
				violations = append(violations,
					filepath.Base(pos.Filename)+":"+
						strings.TrimPrefix(pos.String(), pos.Filename)+
						" calls translator.NewFactory() — domain objects must be injected, not constructed in adapters")
			}
			return true
		})
	}

	if len(violations) > 0 {
		t.Errorf("SRP/DIP violation: adapters construct domain objects directly:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// TestPortInterfacesShouldOnlyUseTypedModels checks that port interfaces
// do not expose raw map[string]interface{} parameters or return values,
// which would leak infrastructure wire-format concerns into the domain boundary.
func TestPortInterfacesShouldOnlyUseTypedModels(t *testing.T) {
	root := projectRoot(t)
	portsDir := filepath.Join(root, "internal", "ports")

	files, err := collectGoFiles(portsDir)
	if err != nil {
		t.Fatalf("cannot walk ports directory: %v", err)
	}

	violations := []string{}
	for _, path := range files {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("cannot parse %s: %v", path, err)
		}

		ast.Inspect(f, func(n ast.Node) bool {
			// Look for method signatures in interface types
			field, ok := n.(*ast.Field)
			if !ok {
				return true
			}
			funcType, ok := field.Type.(*ast.FuncType)
			if !ok {
				return true
			}

			// Collect all parameter and result types as source text
			checkFields := func(fl *ast.FieldList, direction string) {
				if fl == nil {
					return
				}
				for _, f := range fl.List {
					typeStr := formatType(f.Type)
					if strings.Contains(typeStr, "map[string]interface{}") ||
						strings.Contains(typeStr, "interface{}") ||
						strings.Contains(typeStr, "any") {
						methodName := ""
						if len(field.Names) > 0 {
							methodName = field.Names[0].Name
						}
						pos := fset.Position(f.Pos())
						violations = append(violations,
							filepath.Base(pos.Filename)+": method "+methodName+
								" uses map[string]interface{} in "+direction+
								" — ports should use typed domain structs")
					}
				}
			}

			checkFields(funcType.Params, "parameters")
			checkFields(funcType.Results, "return values")
			return true
		})

	}

	if len(violations) > 0 {
		t.Errorf("ISP/hexagonal violation: port interfaces expose raw map types:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// formatType returns a string representation of an AST type expression,
// sufficient to detect map[string]interface{} patterns.
func formatType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.MapType:
		return "map[" + formatType(t.Key) + "]" + formatType(t.Value)
	case *ast.InterfaceType:
		if t.Methods == nil || t.Methods.NumFields() == 0 {
			return "interface{}"
		}
		return "interface{...}"
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return formatType(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + formatType(t.X)
	case *ast.ArrayType:
		return "[]" + formatType(t.Elt)
	case *ast.SliceExpr:
		return "[]"
	default:
		return ""
	}
}

// TestContextShouldBeFirstParam verifies the Go convention that context.Context
// is always the first parameter when present. This is also a BestPractices rule
// in newer archunit versions, but we enforce it manually here for domain and ports.
func TestContextShouldBeFirstParam(t *testing.T) {
	root := projectRoot(t)
	dirs := []string{
		filepath.Join(root, "internal", "ports"),
		filepath.Join(root, "internal", "domain", "service"),
	}

	violations := []string{}
	for _, dir := range dirs {
		files, err := collectGoFiles(dir)
		if err != nil {
			t.Fatalf("cannot walk %s: %v", dir, err)
		}

		for _, path := range files {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("cannot parse %s: %v", path, err)
			}

			ast.Inspect(f, func(n ast.Node) bool {
				fn, ok := n.(*ast.FuncDecl)
				if !ok || fn.Type.Params == nil || fn.Type.Params.NumFields() == 0 {
					return true
				}

				// Check if any parameter (not first) is context.Context
				params := fn.Type.Params.List
				for i := 1; i < len(params); i++ {
					if sel, ok := params[i].Type.(*ast.SelectorExpr); ok {
						if ident, ok := sel.X.(*ast.Ident); ok {
							if ident.Name == "context" && sel.Sel.Name == "Context" {
								pos := fset.Position(fn.Pos())
								violations = append(violations,
									filepath.Base(pos.Filename)+": func "+fn.Name.Name+
										" — context.Context must be the first parameter")
							}
						}
					}
				}
				return true
			})
		}
	}

	if len(violations) > 0 {
		t.Errorf("Go convention violation: context.Context is not the first parameter:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// TestTranslatorStrategiesMustImplementInterface verifies that every *Strategy
// struct in the translator package actually implements the Translator interface
// by checking that all three required methods are present (ToHue, ToHA, GetMetadata).
// This is a lightweight OCP/LSP guard: if someone adds a strategy without all methods,
// this test fails before the compiler does (useful in early TDD cycles).
func TestTranslatorStrategiesMustImplementInterface(t *testing.T) {
	root := projectRoot(t)
	translatorDir := filepath.Join(root, "internal", "domain", "translator")

	files, err := collectGoFiles(translatorDir)
	if err != nil {
		t.Fatalf("cannot walk translator directory: %v", err)
	}

	requiredMethods := map[string]bool{
		"ToHue":       false,
		"ToHA":        false,
		"GetMetadata": false,
	}

	// Map: strategyTypeName -> set of implemented methods
	strategies := map[string]map[string]bool{}

	for _, path := range files {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("cannot parse %s: %v", path, err)
		}

		ast.Inspect(f, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Recv.NumFields() == 0 {
				return true
			}

			// Get receiver type name
			recv := fn.Recv.List[0].Type
			var typeName string
			if star, ok := recv.(*ast.StarExpr); ok {
				if ident, ok := star.X.(*ast.Ident); ok {
					typeName = ident.Name
				}
			}

			if !strings.HasSuffix(typeName, "Strategy") {
				return true
			}

			if _, exists := strategies[typeName]; !exists {
				strategies[typeName] = map[string]bool{}
			}
			strategies[typeName][fn.Name.Name] = true
			return true
		})
	}

	if len(strategies) == 0 {
		t.Error("No *Strategy types found in translator package — at least one is required")
		return
	}

	violations := []string{}
	for stratName, methods := range strategies {
		for required := range requiredMethods {
			if !methods[required] {
				violations = append(violations,
					stratName+" is missing method "+required+" (required by Translator interface)")
			}
		}
	}

	if len(violations) > 0 {
		t.Errorf("LSP violation: Strategy types do not fully implement Translator interface:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// TestNoCyclicDependenciesBetweenDomainPackages ensures there are no import cycles
// between domain sub-packages (model, service, translator).
// Allowed: service -> model, translator -> model
// Forbidden: model -> service, model -> translator, translator -> service, service -> translator
func TestNoCyclicDependenciesBetweenDomainPackages(t *testing.T) {
	root := projectRoot(t)

	// Forbidden import directions within domain
	forbidden := []struct {
		from string
		to   string
	}{
		{"domain/model", "domain/service"},
		{"domain/model", "domain/translator"},
		{"domain/translator", "domain/service"},
		{"domain/service", "domain/translator"},
	}

	allFiles, err := collectGoFiles(filepath.Join(root, "internal", "domain"))
	if err != nil {
		t.Fatalf("cannot walk domain directory: %v", err)
	}

	violations := []string{}
	for _, path := range allFiles {
		imports, err := importsOf(path)
		if err != nil {
			t.Fatalf("cannot parse imports of %s: %v", path, err)
		}

		// Determine which sub-package this file belongs to
		relPath := strings.TrimPrefix(path, filepath.Join(root, "internal")+"/")
		// e.g. "domain/service/bridge.go" -> "domain/service"
		pkgPath := filepath.Dir(relPath)

		for _, rule := range forbidden {
			if !strings.Contains(pkgPath, strings.ReplaceAll(rule.from, "/", string(os.PathSeparator))) {
				continue
			}
			for imp := range imports {
				if strings.Contains(imp, rule.to) {
					violations = append(violations,
						path+" ("+rule.from+") imports "+imp+" ("+rule.to+") — forbidden direction")
				}
			}
		}
	}

	if len(violations) > 0 {
		t.Errorf("Cyclic/forbidden domain dependency:\n  %s",
			strings.Join(violations, "\n  "))
	}
}
