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

func TestIssueOpsBaseSyncPortOwnsOnlyRequestReceiptAndInterface(t *testing.T) {
	dir := filepath.Join(findRepoRoot(t), "internal", "port", "issueopsbasesync")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	allowedTypes := map[string]bool{"Request": true, "Receipt": true, "Inspector": true}
	violations := []string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, entry.Name()), nil, 0)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		for _, declaration := range parsed.Decls {
			switch typed := declaration.(type) {
			case *ast.GenDecl:
				for _, spec := range typed.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if ok && ast.IsExported(typeSpec.Name.Name) && !allowedTypes[typeSpec.Name.Name] {
						violations = append(violations, entry.Name()+": type "+typeSpec.Name.Name)
					}
				}
			case *ast.FuncDecl:
				if ast.IsExported(typed.Name.Name) {
					violations = append(violations, entry.Name()+": func "+typed.Name.Name)
				}
			}
		}
		for _, imported := range parsed.Imports {
			name := strings.Trim(imported.Path.Value, `"`)
			if name != "context" {
				violations = append(violations, entry.Name()+": import "+name)
			}
		}
	}
	sort.Strings(violations)
	if len(violations) != 0 {
		t.Fatalf("issueops base-sync port owns behavior outside request/receipt/interface: %s", strings.Join(violations, ", "))
	}
}
