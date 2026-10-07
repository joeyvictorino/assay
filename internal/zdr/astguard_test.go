package zdr

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fileWriters are the os functions that create or modify files.
var fileWriters = map[string]bool{
	"Create":    true,
	"WriteFile": true,
	"OpenFile":  true,
	"MkdirAll":  true,
}

// allowedWriterPackages may call fileWriters in non-test code. Everything
// else under internal/ is forbidden from touching the filesystem for writing.
var allowedWriterPackages = map[string]bool{
	"internal/audit":     true,
	"internal/report":    true,
	"internal/remediate": true,
	"internal/cli":       true,
	"internal/results":   true,
}

// TestNoFileWritesOutsideAllowedPackages walks every non-test Go file under
// internal/ and fails if os.Create, os.WriteFile, os.OpenFile or os.MkdirAll
// is called from a package not in allowedWriterPackages. Test files are
// exempt: they write fixtures to t.TempDir().
func TestNoFileWritesOutsideAllowedPackages(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	internalDir := filepath.Join(root, "internal")
	var violations []string
	checked := 0
	err = filepath.WalkDir(internalDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		pkg := filepath.ToSlash(filepath.Dir(rel))
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		checked++
		osNames := osImportNames(f)
		if len(osNames) == 0 {
			return nil
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
			id, ok := sel.X.(*ast.Ident)
			if !ok || !osNames[id.Name] || !fileWriters[sel.Sel.Name] {
				return true
			}
			if !allowedWriterPackages[pkg] {
				violations = append(violations, fset.Position(call.Pos()).String()+": os."+sel.Sel.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no Go files checked; wrong root?")
	}
	sort.Strings(violations)
	for _, v := range violations {
		t.Errorf("file write outside allowed packages: %s", v)
	}
}

// osImportNames returns the local names under which "os" is imported.
func osImportNames(f *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, imp := range f.Imports {
		if strings.Trim(imp.Path.Value, `"`) != "os" {
			continue
		}
		if imp.Name != nil {
			if imp.Name.Name == "_" {
				continue
			}
			names[imp.Name.Name] = true
		} else {
			names["os"] = true
		}
	}
	return names
}

// TestAstGuardCatchesViolations makes sure the guard logic itself works by
// running it on an in-memory file.
func TestAstGuardCatchesViolations(t *testing.T) {
	src := `package x
import (
	"os"
	myos "os"
	_ "os"
)
func f() {
	os.Create("a")
	myos.WriteFile("b", nil, 0)
	os.ReadFile("c")
	os.Remove("d")
}`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	names := osImportNames(f)
	if !names["os"] || !names["myos"] || len(names) != 2 {
		t.Fatalf("names %v", names)
	}
	var found []string
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && names[id.Name] && fileWriters[sel.Sel.Name] {
					found = append(found, sel.Sel.Name)
				}
			}
		}
		return true
	})
	if strings.Join(found, ",") != "Create,WriteFile" {
		t.Fatalf("found %v", found)
	}
}
