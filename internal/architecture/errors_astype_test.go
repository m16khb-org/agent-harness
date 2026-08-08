package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// production 코드는 errors.As의 out-parameter 형태 대신 errors.AsType의 값+ok 형태를
// 쓴다. 두 형태가 섞이면 다음 사람이 어느 쪽이 이 저장소의 관례인지 알 수 없다(#428).
// 타입이 아니라 값을 매칭해야 하는 호출부가 생기면 errors.Is가 그 축이며, AsType으로
// 옮길 수 없는 호출부가 생기면 사유를 주석으로 남기고 이 규칙을 명시적으로 넓힌다.
func TestProductionCodeUsesErrorsAsTypeInsteadOfErrorsAs(t *testing.T) {
	repoRoot := findRepoRoot(t)
	var violations []string
	for _, root := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(repoRoot, root), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			errorsPackage := standardErrorsIdentifier(file)
			if errorsPackage == "" {
				return nil
			}
			relative, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "As" {
					return true
				}
				identifier, ok := selector.X.(*ast.Ident)
				if !ok || identifier.Name != errorsPackage {
					return true
				}
				violations = append(violations, relative+":"+strconv.Itoa(fset.Position(call.Pos()).Line))
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(violations)
	if len(violations) != 0 {
		t.Fatalf("production code still calls errors.As instead of errors.AsType: %s", strings.Join(violations, ", "))
	}
}

// import alias를 쓴 파일에서도 표준 errors 호출을 찾기 위해 실제 바인딩 이름을 읽는다.
func standardErrorsIdentifier(file *ast.File) string {
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil || path != "errors" {
			continue
		}
		if imported.Name != nil {
			return imported.Name.Name
		}
		return "errors"
	}
	return ""
}
