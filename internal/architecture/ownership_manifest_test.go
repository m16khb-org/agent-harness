package architecture

import (
	"strings"
	"testing"
)

func TestOwnershipManifestRejectsLegacyAndInvertedEdges(t *testing.T) {
	tests := []struct {
		name string
		edge dependencyEdge
		rule string
	}{
		{"core package", dependencyEdge{"internal/core/state", "fmt"}, "ownership_forbids_core_package"},
		{"core import", dependencyEdge{"internal/contract/state", "internal/core/state"}, "ownership_forbids_core_package"},
		{"domain adapter", dependencyEdge{"internal/domain/state", "internal/adapter/fs"}, "domain_must_only_import_contract"},
		{"application command", dependencyEdge{"internal/application/nativeactivation", "cmd/harness"}, "application_must_not_import_adapter_or_cmd"},
		{"contract domain", dependencyEdge{"internal/contract/state", "internal/domain/state"}, "contract_must_not_import_internal"},
		{"port contract", dependencyEdge{"internal/port/state", "internal/contract/state"}, "port_must_not_import_internal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := evaluateOwnershipEdges([]dependencyEdge{tt.edge})
			if !containsViolation(violations, tt.rule, tt.edge) {
				t.Fatalf("expected %s for %s, got %v", tt.rule, formatEdge(tt.edge), violations)
			}
		})
	}
}

func TestOwnershipManifestAllowsTargetDirections(t *testing.T) {
	edges := []dependencyEdge{
		{"internal/contract/state", "errors"},
		{"internal/domain/state", "internal/contract/state"},
		{"internal/application/nativeactivation", "internal/domain/state"},
		{"internal/application/nativeactivation", "internal/contract/nativeactivation"},
		{"internal/application/nativeactivation", "internal/port/nativeactivation"},
		{"internal/adapter/outbound/nativeactivation", "internal/application/nativeactivation"},
		{"cmd/harness/harnessapp", "internal/adapter/outbound/nativeactivation"},
	}
	if violations := evaluateOwnershipEdges(edges); len(violations) != 0 {
		t.Fatalf("target ownership directions must be allowed, got %s", formatViolations(violations))
	}
}

func TestProductionFoundationOwnershipHasNoForbiddenEdges(t *testing.T) {
	if violations := foundationOwnershipViolations(loadProductionEdges(t)); len(violations) != 0 {
		t.Fatalf("foundation ownership violations:\n%s", formatViolations(violations))
	}
	if violations := foundationPackageViolations(loadProductionPackages(t)); len(violations) != 0 {
		t.Fatalf("foundation package violations:\n%s", strings.Join(violations, "\n"))
	}
}

func TestFoundationOwnershipRejectsNestedCoreModelPrefix(t *testing.T) {
	edge := dependencyEdge{"internal/core/issueops/model/nested", "fmt"}
	if violations := foundationOwnershipViolations([]dependencyEdge{edge}); !containsViolation(violations, "ownership_forbids_core_package", edge) {
		t.Fatalf("nested model prefix was not rejected: %v", violations)
	}
}
