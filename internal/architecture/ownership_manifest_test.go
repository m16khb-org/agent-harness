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
		{"port domain", dependencyEdge{"internal/port/state", "internal/domain/state"}, "port_must_not_import_internal"},
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
		// DTO가 다른 capability의 DTO를 필드로 갖는 것은 계약 조합이지 구현 의존이
		// 아니다. contract 사이 참조는 이미 lease/completion/preparation vertical이
		// 쓰고 있는 방향이다.
		{"internal/contract/issueopslease", "internal/contract/state"},
		{"internal/contract/lifecycle", "internal/contract/projectdoc"},
		// port는 계약 어휘로 말한다. 인터페이스가 DTO를 시그니처에 쓰는 것은
		// 구현 의존이 아니다.
		{"internal/port/issueopsbasesync", "internal/contract/issueops"},
		{"internal/port", "internal/contract/state"},
	}
	if violations := evaluateOwnershipEdges(edges); len(violations) != 0 {
		t.Fatalf("target ownership directions must be allowed, got %s", formatViolations(violations))
	}
}

// contract가 구현 계층을 참조하는 것은 여전히 금지다. 위 예외는 contract 사이에만
// 적용되며 domain·application·adapter·cmd로 향하는 edge는 그대로 막힌다.
func TestOwnershipManifestStillRejectsContractToImplementation(t *testing.T) {
	for _, imported := range []string{
		"internal/domain/state",
		"internal/application/state",
		"internal/adapter/outbound/state",
		"internal/port/state",
	} {
		edge := dependencyEdge{"internal/contract/state", imported}
		if !containsViolation(evaluateOwnershipEdges([]dependencyEdge{edge}), "contract_must_not_import_internal", edge) {
			t.Fatalf("contract must not import %s", imported)
		}
	}
}

// port가 구현 계층을 참조하는 것은 여전히 금지다. 위 예외는 contract와 port
// 사이에만 적용된다.
func TestOwnershipManifestStillRejectsPortToImplementation(t *testing.T) {
	for _, imported := range []string{
		"internal/domain/state",
		"internal/application/state",
		"internal/adapter/outbound/state",
	} {
		edge := dependencyEdge{"internal/port", imported}
		if !containsViolation(evaluateOwnershipEdges([]dependencyEdge{edge}), "port_must_not_import_internal", edge) {
			t.Fatalf("port must not import %s", imported)
		}
	}
}

// 공유 저장 엔진 예외는 outbound -> sqlstore 한 방향뿐이다. cmd나 inbound에서
// 엔진을 직접 잡는 것은 그대로 위반이다.
func TestSharedStorageEngineExceptionIsOneDirectionOnly(t *testing.T) {
	if !isSharedStorageEngineEdge("internal/adapter/outbound/state", "internal/adapter/outbound/sqlstore") {
		t.Fatal("outbound adapters may use the shared storage engine")
	}
	for _, importer := range []string{
		"cmd/harness/statecli",
		"internal/adapter/inbound/issueopslease",
		"internal/domain/state",
	} {
		if isSharedStorageEngineEdge(importer, "internal/adapter/outbound/sqlstore") {
			t.Fatalf("%s must not reach the storage engine directly", importer)
		}
	}
	if isSharedStorageEngineEdge("internal/adapter/outbound/state", "internal/adapter/lifecycle") {
		t.Fatal("the exception must not widen beyond the storage engine")
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
