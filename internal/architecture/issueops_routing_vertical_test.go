package architecture

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestIssueOpsRoutingVerticalOwnsLiveRouting(t *testing.T) {
	requiredPackages := []string{
		"internal/contract/issueopsrouting",
		"internal/domain/issueopsrouting",
		"internal/application/issueopsrouting",
		"internal/adapter/inbound/issueopsrouting",
		"internal/adapter/outbound/issueopsrouting",
	}
	productionPackages := loadProductionPackages(t)
	for _, required := range requiredPackages {
		if !slices.Contains(productionPackages, required) {
			t.Errorf("missing issueops routing package %s", required)
		}
	}

	legacyPath := filepath.Join(
		findRepoRoot(t),
		"internal",
		"adapter",
		"issueops",
		"issueops_routing.go",
	)
	if _, err := os.Stat(legacyPath); err == nil {
		t.Errorf("legacy routing implementation must be deleted: %s", legacyPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inspect legacy routing implementation: %v", err)
	}

	for _, edge := range loadProductionEdges(t) {
		if !strings.Contains(edge.importer, "issueopsrouting") {
			continue
		}
		if edge.imported == "internal/adapter/issueops" ||
			strings.HasPrefix(edge.imported, "internal/adapter/issueops/") {
			t.Errorf("issueops routing vertical imports legacy adapter: %s", formatEdge(edge))
		}
	}
}
