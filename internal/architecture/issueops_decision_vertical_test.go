package architecture

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestIssueOpsDecisionVerticalOwnsDecisionRecording(t *testing.T) {
	requiredPackages := []string{
		"internal/contract/issueopsdecision",
		"internal/domain/issueopsdecision",
		"internal/application/issueopsdecision",
		"internal/adapter/inbound/issueopsdecision",
		"internal/adapter/outbound/issueopsdecision",
	}
	productionPackages := loadProductionPackages(t)
	for _, required := range requiredPackages {
		if !slices.Contains(productionPackages, required) {
			t.Errorf("missing issueops decision package %s", required)
		}
	}

	legacyPath := filepath.Join(
		findRepoRoot(t),
		"internal",
		"adapter",
		"issueops",
		"issueops_decision.go",
	)
	if _, err := os.Stat(legacyPath); err == nil {
		t.Errorf("legacy decision implementation must be deleted: %s", legacyPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inspect legacy decision implementation: %v", err)
	}

	for _, edge := range loadProductionEdges(t) {
		if !strings.Contains(edge.importer, "issueopsdecision") {
			continue
		}
		if edge.imported == "internal/adapter/issueops" ||
			strings.HasPrefix(edge.imported, "internal/adapter/issueops/") {
			t.Errorf("issueops decision vertical imports legacy adapter: %s", formatEdge(edge))
		}
	}
}
