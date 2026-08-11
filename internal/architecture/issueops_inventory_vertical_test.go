package architecture

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestIssueOpsInventoryVerticalOwnsCycleListing(t *testing.T) {
	repoRoot := findRepoRoot(t)
	requiredPackages := []string{
		"internal/contract/issueopsinventory",
		"internal/domain/issueopsinventory",
		"internal/application/issueopsinventory",
		"internal/adapter/inbound/issueopsinventory",
		"internal/adapter/outbound/issueopsinventory",
	}
	productionPackages := loadProductionPackages(t)
	for _, required := range requiredPackages {
		if !slices.Contains(productionPackages, required) {
			t.Errorf("missing issueops inventory package %s", required)
		}
	}

	legacyPath := filepath.Join(repoRoot, "internal", "adapter", "issueops", "issueops_list.go")
	if _, err := os.Stat(legacyPath); err == nil {
		t.Errorf("legacy issueops cycle listing remains at %s", legacyPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inspect legacy issueops cycle listing: %v", err)
	}

	for _, edge := range loadProductionEdges(t) {
		if !strings.Contains(edge.importer, "issueopsinventory") {
			continue
		}
		if edge.imported == "internal/adapter/issueops" ||
			strings.HasPrefix(edge.imported, "internal/adapter/issueops/") {
			t.Errorf("issueops inventory vertical imports legacy adapter: %s", formatEdge(edge))
		}
	}
}
