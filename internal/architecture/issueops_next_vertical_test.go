package architecture

import (
	"slices"
	"strings"
	"testing"
)

// TestIssueOpsNextVerticalOwnsStageClassification은 단계 분류가 legacy
// 어댑터로 새어 들어가는 것을 막는다. 분류는 순수 함수여야 재현 가능하고,
// 그 위 계층만 관측을 주입한다.
func TestIssueOpsNextVerticalOwnsStageClassification(t *testing.T) {
	requiredPackages := []string{
		"internal/contract/issueopsnext",
		"internal/domain/issueopsnext",
		"internal/application/issueopsnext",
		"internal/adapter/inbound/issueopsnext",
	}
	productionPackages := loadProductionPackages(t)
	for _, required := range requiredPackages {
		if !slices.Contains(productionPackages, required) {
			t.Errorf("missing issueops next package %s", required)
		}
	}

	for _, edge := range loadProductionEdges(t) {
		if !slices.Contains(requiredPackages, edge.importer) {
			continue
		}
		if edge.imported == "internal/adapter/issueops" ||
			strings.HasPrefix(edge.imported, "internal/adapter/issueops/") {
			t.Errorf("issueops next vertical imports legacy adapter: %s", formatEdge(edge))
		}
	}
}
