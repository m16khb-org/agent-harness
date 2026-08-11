package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestIssueOpsStatusVerticalOwnsStatusProjection(t *testing.T) {
	requiredPackages := []string{
		"internal/contract/issueopsstatus",
		"internal/domain/issueopsstatus",
		"internal/application/issueopsstatus",
		"internal/adapter/inbound/issueopsstatus",
		"internal/adapter/outbound/issueopsstatus",
	}
	productionPackages := loadProductionPackages(t)
	for _, required := range requiredPackages {
		if !slices.Contains(productionPackages, required) {
			t.Errorf("missing issueops status package %s", required)
		}
	}

	legacyPath := filepath.Join(
		findRepoRoot(t),
		"internal",
		"adapter",
		"issueops",
		"issueops_phase_ledger.go",
	)
	file, err := parser.ParseFile(token.NewFileSet(), legacyPath, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		switch function.Name.Name {
		case "IssueOpsStatus", "DeriveIssueOpsPhaseLedger", "issueOpsPhaseArtifactKeys":
			t.Errorf("legacy status projection symbol remains: %s", function.Name.Name)
		}
	}

	for _, edge := range loadProductionEdges(t) {
		if !strings.Contains(edge.importer, "issueopsstatus") {
			continue
		}
		if edge.imported == "internal/adapter/issueops" ||
			strings.HasPrefix(edge.imported, "internal/adapter/issueops/") {
			t.Errorf("issueops status vertical imports legacy adapter: %s", formatEdge(edge))
		}
	}
}
