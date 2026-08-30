package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestInstallPlanHasSingleCanonicalDeclaration(t *testing.T) {
	repoRoot := findRepoRoot(t)
	var canonical []string
	var invalid []string
	for _, root := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(repoRoot, root), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			for _, declaration := range file.Decls {
				general, ok := declaration.(*ast.GenDecl)
				if !ok || general.Tok != token.TYPE {
					continue
				}
				for _, spec := range general.Specs {
					named := spec.(*ast.TypeSpec)
					if named.Name.Name == "InstallPlan" {
						relative, err := filepath.Rel(repoRoot, path)
						if err != nil {
							return err
						}
						relativePath := filepath.ToSlash(relative)
						if relativePath == "internal/port/install.go" && !named.Assign.IsValid() {
							canonical = append(canonical, relativePath)
							continue
						}
						selector, selectorOK := named.Type.(*ast.SelectorExpr)
						packageName, packageOK := selector.X.(*ast.Ident)
						if !named.Assign.IsValid() || !selectorOK || !packageOK ||
							packageName.Name != "port" || selector.Sel.Name != "InstallPlan" {
							invalid = append(invalid, relativePath)
						}
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(canonical)
	sort.Strings(invalid)
	if got, want := strings.Join(canonical, "\n"), "internal/port/install.go"; got != want {
		t.Fatalf("InstallPlan must have one canonical declaration in the port, got:\n%s", got)
	}
	if len(invalid) > 0 {
		t.Fatalf("InstallPlan adapters must alias port.InstallPlan, got:\n%s", strings.Join(invalid, "\n"))
	}
}
