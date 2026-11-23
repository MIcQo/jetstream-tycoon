// Command simlint enforces simulation determinism rules in sim/ (ADR-0006, ADR-0021):
//   - No range over maps
//   - No goroutines (go statements) or select statements
//   - No imports of time, math/rand, or math/rand/v2
//   - No math.* calls beyond Sqrt, Floor, Abs, Min, Max, IsNaN, IsInf, Float64bits, Float64frombits
package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var allowedMathFuncs = map[string]bool{
	"Sqrt":            true,
	"Floor":           true,
	"Abs":             true,
	"Min":             true,
	"Max":             true,
	"IsNaN":           true,
	"IsInf":           true,
	"Float64bits":     true,
	"Float64frombits": true,
}

var forbiddenImports = map[string]string{
	"time":         "time is forbidden in sim/ (use tick count, ADR-0006)",
	"math/rand":    "math/rand is forbidden in sim/ (use per-system PCG stream, ADR-0006)",
	"math/rand/v2": "math/rand/v2 is forbidden in sim/ (use per-system PCG stream, ADR-0006)",
}

func main() {
	root := "sim"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	issues, err := CheckDir(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "simlint error: %v\n", err)
		os.Exit(2)
	}
	if len(issues) > 0 {
		for _, issue := range issues {
			fmt.Fprintln(os.Stderr, issue)
		}
		os.Exit(1)
	}
}

// CheckDir walks root and checks every Go package under it.
func CheckDir(root string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var allIssues []string
	for _, dir := range dirs {
		issues, err := checkPackageDir(dir)
		if err != nil {
			return nil, err
		}
		allIssues = append(allIssues, issues...)
	}
	return allIssues, nil
}

func checkPackageDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		fullPath := filepath.Join(dir, e.Name())
		f, err := parser.ParseFile(fset, fullPath, nil, 0)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		return nil, nil
	}

	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{
		Importer:                 importer.Default(),
		DisableUnusedImportCheck: true,
		Error:                    func(error) {},
	}
	_, _ = conf.Check(dir, fset, files, info)

	var issues []string
	for _, f := range files {
		issues = append(issues, InspectFile(fset, f, info)...)
	}
	return issues, nil
}

// InspectFile checks a single parsed file against ADR-0006 and ADR-0021 rules.
func InspectFile(fset *token.FileSet, f *ast.File, info *types.Info) []string {
	var issues []string
	mathAliases := make(map[string]bool)

	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if msg, bad := forbiddenImports[path]; bad {
			pos := fset.Position(imp.Pos())
			issues = append(issues, fmt.Sprintf("%s: %s", pos, msg))
		}
		if path == "math" {
			name := "math"
			if imp.Name != nil {
				name = imp.Name.Name
			}
			mathAliases[name] = true
		}
	}

	ast.Inspect(f, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.GoStmt:
			pos := fset.Position(stmt.Pos())
			issues = append(issues, fmt.Sprintf("%s: goroutines (go statement) are forbidden in sim/ (ADR-0006)", pos))
		case *ast.SelectStmt:
			pos := fset.Position(stmt.Pos())
			issues = append(issues, fmt.Sprintf("%s: select statements are forbidden in sim/ (ADR-0006)", pos))
		case *ast.RangeStmt:
			if info != nil {
				if tv, ok := info.Types[stmt.X]; ok && tv.Type != nil {
					if _, isMap := tv.Type.Underlying().(*types.Map); isMap {
						pos := fset.Position(stmt.Pos())
						issues = append(issues, fmt.Sprintf("%s: ranging over a map is forbidden in sim/ (iterate sorted keys, ADR-0006)", pos))
					}
				}
			}
		case *ast.CallExpr:
			if sel, ok := stmt.Fun.(*ast.SelectorExpr); ok {
				if pkgIdent, ok := sel.X.(*ast.Ident); ok && mathAliases[pkgIdent.Name] {
					if !allowedMathFuncs[sel.Sel.Name] {
						pos := fset.Position(sel.Pos())
						issues = append(issues, fmt.Sprintf("%s: math.%s is forbidden in sim/ (use sim/detmath, ADR-0021)", pos, sel.Sel.Name))
					}
				}
			}
		}
		return true
	})

	return issues
}
