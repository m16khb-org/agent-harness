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

func TestIssueOpsArtifactVerticalOwnsStaging(t *testing.T) {
	requiredPackages := []string{
		"internal/contract/issueopsartifact",
		"internal/domain/issueopsartifact",
		"internal/application/issueopsartifact",
		"internal/adapter/inbound/issueopsartifact",
		"internal/adapter/outbound/issueopsartifact",
	}
	productionPackages := loadProductionPackages(t)
	for _, required := range requiredPackages {
		if !slices.Contains(productionPackages, required) {
			t.Errorf("missing issueops artifact package %s", required)
		}
	}

	legacyPath := filepath.Join(
		findRepoRoot(t),
		"internal",
		"adapter",
		"issueops",
		"issueops_artifact_stage.go",
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
		case "StageIssueOpsArtifact",
			"UnstageIssueOpsArtifact",
			"StagedIssueOpsArtifactNames",
			"canStageIssueOpsArtifact",
			"rejectSecretLikeContent":
			t.Errorf("legacy artifact staging symbol remains: %s", function.Name.Name)
		}
	}

	for _, edge := range loadProductionEdges(t) {
		if !strings.Contains(edge.importer, "issueopsartifact") {
			continue
		}
		if edge.imported == "internal/adapter/issueops" ||
			strings.HasPrefix(edge.imported, "internal/adapter/issueops/") {
			t.Errorf("issueops artifact vertical imports legacy adapter: %s", formatEdge(edge))
		}
	}
}
