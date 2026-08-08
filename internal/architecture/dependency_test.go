package architecture

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type dependencyEdge struct {
	importer string
	imported string
}

type violation struct {
	rule string
	edge dependencyEdge
}

func TestEvaluateEdgesRejectsForbiddenDependencies(t *testing.T) {
	tests := []struct {
		name string
		edge dependencyEdge
		rule string
	}{
		{"root core adapter", dependencyEdge{"internal/core", "internal/adapter"}, "core_must_not_import_adapter_or_cmd"},
		{"core adapter", dependencyEdge{"internal/core/issueops", "internal/adapter/provider"}, "core_must_not_import_adapter_or_cmd"},
		{"core command", dependencyEdge{"internal/core/issueops", "cmd/harness"}, "core_must_not_import_adapter_or_cmd"},
		{"root adapter command", dependencyEdge{"internal/adapter", "cmd"}, "adapter_must_not_import_cmd"},
		{"adapter command", dependencyEdge{"internal/adapter/cli", "cmd/harness"}, "adapter_must_not_import_cmd"},
		{"port internal", dependencyEdge{"internal/port", "internal/core"}, "port_must_not_import_internal"},
		{"domain os", dependencyEdge{"internal/domain/session", "os"}, "domain_must_not_import_implementation"},
		{"domain net", dependencyEdge{"internal/domain/session", "net"}, "domain_must_not_import_implementation"},
		{"domain sqlite", dependencyEdge{"internal/domain/session", "modernc.org/sqlite"}, "domain_must_not_import_implementation"},
		{"domain contract root", dependencyEdge{"internal/domain/session", "internal/contract"}, "domain_must_not_import_implementation"},
		{"application outbound adapter", dependencyEdge{"internal/application/run", "internal/adapter/provider"}, "application_must_not_import_implementation"},
		{"release application filesystem", dependencyEdge{"internal/application/issueopslease", "path/filepath"}, "application_must_not_import_implementation"},
		{"release contract production issueops", dependencyEdge{"internal/contract/issueopslease", "internal/core/issueops/model"}, "leasevertical_contract_must_not_import_production_issueops"},
		{"publication contract core", dependencyEdge{"internal/contract/issueopspublication", "internal/core/issueops"}, "publication_contract_must_not_import_internal"},
		{"publication contract database", dependencyEdge{"internal/contract/issueopspublication", "database/sql"}, "publication_contract_must_not_import_internal"},
		{"publication domain port", dependencyEdge{"internal/domain/issueopspublication", "internal/port"}, "publication_domain_must_only_import_contract"},
		{"publication application port", dependencyEdge{"internal/application/issueopspublication", "internal/port"}, "publication_application_must_only_import_domain_or_contract"},
		{"publication outbound core", dependencyEdge{"internal/adapter/outbound/issueopspublication", "internal/core/issueops"}, "publication_outbound_adapter_must_not_import_core"},
		{"completion contract core", dependencyEdge{"internal/contract/issueopscompletion", "internal/core/issueops"}, "completion_contract_must_only_import_stable_contract"},
		{"completion domain port", dependencyEdge{"internal/domain/issueopscompletion", "internal/port"}, "completion_domain_must_only_import_contract"},
		{"completion application port", dependencyEdge{"internal/application/issueopscompletion", "internal/port"}, "completion_application_must_only_import_domain_or_contract"},
		{"completion outbound core", dependencyEdge{"internal/adapter/outbound/issueopscompletion", "internal/core/issueops"}, "completion_outbound_adapter_must_not_import_core"},
		{"preparation contract core", dependencyEdge{"internal/contract/issueopspreparation", "internal/core/issueops"}, "preparation_contract_must_only_import_lease_contract"},
		{"preparation domain port", dependencyEdge{"internal/domain/issueopspreparation", "internal/port"}, "preparation_domain_must_only_import_contract"},
		{"preparation application port", dependencyEdge{"internal/application/issueopspreparation", "internal/port"}, "preparation_application_must_only_import_domain_or_contract"},
		{"preparation outbound core", dependencyEdge{"internal/adapter/outbound/issueopspreparation", "internal/core/issueops"}, "preparation_outbound_adapter_must_not_import_core"},
		{"application syscall", dependencyEdge{"internal/application/run", "syscall"}, "application_must_not_import_implementation"},
		{"inbound adapter outbound adapter", dependencyEdge{"internal/adapter/inbound/http", "internal/adapter/outbound/github"}, "inbound_adapter_must_not_import_outbound_adapter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := evaluateEdges([]dependencyEdge{tt.edge})
			if !containsViolation(violations, tt.rule, tt.edge) {
				t.Fatalf("expected %s for %s -> %s, got %v", tt.rule, tt.edge.importer, tt.edge.imported, violations)
			}
			if got, want := formatViolations(violations), tt.rule+": "+formatEdge(tt.edge); got != want {
				t.Fatalf("expected diagnostic %q, got %q", want, got)
			}
		})
	}
}

func TestEvaluateEdgesAllowsPublicationDomainContract(t *testing.T) {
	edge := dependencyEdge{"internal/domain/issueopspublication", "internal/contract/issueopspublication"}
	if violations := evaluateEdges([]dependencyEdge{edge}); len(violations) != 0 {
		t.Fatalf("publication domain contract edge must be allowed, got %v", violations)
	}
}

func TestEvaluateEdgesAllowsCompletionVerticalContracts(t *testing.T) {
	for _, edge := range []dependencyEdge{
		{"internal/domain/issueopscompletion", "internal/contract/issueopscompletion"},
		{"internal/contract/issueopscompletion", "internal/contract/issueopslease"},
	} {
		if violations := evaluateEdges([]dependencyEdge{edge}); len(violations) != 0 {
			t.Fatalf("completion vertical contract edge must be allowed, got %v", violations)
		}
	}
}

func TestEvaluateEdgesAllowsPreparationDomainContract(t *testing.T) {
	edge := dependencyEdge{"internal/domain/issueopspreparation", "internal/contract/issueopspreparation"}
	if violations := evaluateEdges([]dependencyEdge{edge}); len(violations) != 0 {
		t.Fatalf("preparation domain contract edge must be allowed, got %v", violations)
	}
}

func TestEvaluateEdgesAllowsOnlyWaveTwoStorageAdapterImports(t *testing.T) {
	for _, edge := range []dependencyEdge{
		{"internal/core/issueops", "internal/adapter/outbound/sqlstore"},
		{"internal/core/lifecycle", "internal/adapter/outbound/state"},
	} {
		if violations := evaluateEdges([]dependencyEdge{edge}); len(violations) != 0 {
			t.Fatalf("wave-two storage edge %s must remain buildable until the owning core caller moves: %v", formatEdge(edge), violations)
		}
	}
	edge := dependencyEdge{"internal/core/issueops", "internal/adapter/outbound/webfetch"}
	if violations := evaluateEdges([]dependencyEdge{edge}); !containsViolation(violations, "core_must_not_import_adapter_or_cmd", edge) {
		t.Fatalf("unrelated core adapter edge must remain forbidden: %v", violations)
	}
}

func TestCurrentIssueOpsVerticalOnly(t *testing.T) {
	repoRoot := findRepoRoot(t)
	forbidden := []string{
		"Compatibility" + "Oracle",
		"Legacy" + "Orca",
		"Legacy" + "Self",
		"parse" + "Legacy",
		"render" + "Legacy",
		"compatibility" + "Export",
	}
	retiredFacades := map[string]bool{
		"Release" + "Execution":  true,
		"Complete" + "Execution": true,
	}
	var violations []string
	for _, root := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(repoRoot, root), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				return nil
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return err
			}
			for _, identifier := range forbidden {
				if strings.Contains(string(contents), identifier) {
					violations = append(violations, relative+" retains "+identifier)
				}
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, contents, 0)
			if err != nil {
				return err
			}
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if ok && retiredFacades[function.Name.Name] {
					violations = append(violations, relative+" defines retired facade "+function.Name.Name)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(violations) != 0 {
		sort.Strings(violations)
		t.Fatalf("retired IssueOps surfaces remain:\n%s", strings.Join(violations, "\n"))
	}
}

func TestLegacyEdgesClassifyConcreteAdapterOutsideCompositionRoot(t *testing.T) {
	edge := dependencyEdge{"cmd/harness/issueopscli", "internal/adapter/provider"}
	if got := legacyEdges([]dependencyEdge{edge}); !reflect.DeepEqual(got, []dependencyEdge{edge}) {
		t.Fatalf("expected legacy concrete-adapter edge %s, got %v", formatEdge(edge), got)
	}
	compositionRootEdge := dependencyEdge{"cmd/harness/harnessapp", "internal/adapter/provider"}
	if got := legacyEdges([]dependencyEdge{compositionRootEdge}); len(got) != 0 {
		t.Fatalf("expected composition-root edge to stay outside legacy baseline, got %v", got)
	}
	adapterEdge := dependencyEdge{"internal/adapter/cli", "internal/adapter/provider"}
	if got := legacyEdges([]dependencyEdge{adapterEdge}); !reflect.DeepEqual(got, []dependencyEdge{adapterEdge}) {
		t.Fatalf("expected non-composition adapter edge %s in legacy baseline, got %v", formatEdge(adapterEdge), got)
	}
}

func TestLegacyEdgesExcludeSameCapabilityAdapterPackages(t *testing.T) {
	inside := []dependencyEdge{
		{"internal/adapter/issueops", "internal/adapter/issueops/linking"},
		{"internal/adapter/issueops/linking", "internal/adapter/issueops/pathutil"},
		{"internal/adapter/lifecycle/compact", "internal/adapter/lifecycle/model"},
	}
	for _, edge := range inside {
		if got := legacyEdges([]dependencyEdge{edge}); len(got) != 0 {
			t.Fatalf("same-capability adapter edge %s must stay outside the legacy baseline, got %v", formatEdge(edge), got)
		}
	}

	// capability 경계를 넘으면 여전히 legacy다. outbound/inbound는 방향 분류이므로
	// 그 아래 서로 다른 capability는 같은 것으로 묶이지 않는다.
	crossing := []dependencyEdge{
		{"internal/adapter/trace", "internal/adapter/policy"},
		{"internal/adapter/outbound/state", "internal/adapter/outbound/webfetch"},
		{"internal/adapter/lifecycle", "internal/adapter/projectdoc"},
	}
	for _, edge := range crossing {
		if got := legacyEdges([]dependencyEdge{edge}); !reflect.DeepEqual(got, []dependencyEdge{edge}) {
			t.Fatalf("cross-capability adapter edge %s must stay in the legacy baseline, got %v", formatEdge(edge), got)
		}
	}
}

func TestLegacyEdgesExcludeMigratedInboundAdapters(t *testing.T) {
	for _, importer := range []string{"internal/adapter/inbound/issueopslease", "internal/adapter/inbound/issueopspublication", "internal/adapter/inbound/issueopscompletion", "internal/adapter/inbound/issueopspreparation"} {
		edge := dependencyEdge{importer, "internal/core/issueops"}
		if got := legacyEdges([]dependencyEdge{edge}); len(got) != 0 {
			t.Fatalf("migrated inbound edge %s must stay outside the legacy baseline, got %v", formatEdge(edge), got)
		}
	}
}

func TestLegacyInfrastructureIncludesNetAndSyscall(t *testing.T) {
	edges := []dependencyEdge{
		{"internal/core/issueops", "syscall"},
		{"internal/core/issueops", "net"},
	}
	want := []dependencyEdge{
		{"internal/core/issueops", "net"},
		{"internal/core/issueops", "syscall"},
	}
	if got := legacyEdges(edges); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected net and syscall legacy infrastructure edges, got %v", got)
	}
}

func TestProductionGraphHasNoLegacyAdapterEdges(t *testing.T) {
	edges := loadProductionEdges(t)
	if got := evaluateEdges(edges); len(got) != 0 {
		t.Fatalf("forbidden dependency violations:\n%s", formatViolations(got))
	}

	secondInventory := loadProductionEdges(t)
	if !reflect.DeepEqual(edges, secondInventory) {
		t.Fatalf("production import inventory is not byte-stable")
	}

	// legacy baseline은 비었다. 전환이 끝났으므로 래칫은 "남은 edge가 0"이라는
	// 불변식으로 대체한다 — 새 legacy edge는 baseline에 등록하는 것이 아니라
	// 애초에 들어올 수 없다.
	if remaining := legacyEdges(edges); len(remaining) != 0 {
		t.Fatalf("legacy adapter edges are no longer allowed; the transition is complete:\n%s", formatEdges(remaining))
	}
}

func formatEdges(edges []dependencyEdge) string {
	lines := make([]string, 0, len(edges))
	for _, edge := range edges {
		lines = append(lines, "  "+formatEdge(edge))
	}
	return strings.Join(lines, "\n")
}

func TestStateSQLNetworkSourcePrefixesAbsent(t *testing.T) {
	forbidden := []string{
		"internal/core/sqlstore",
		"internal/core/state",
		"internal/core/webfetch",
	}
	var violations []string
	for _, pkg := range loadProductionPackages(t) {
		for _, prefix := range forbidden {
			if pkg == prefix || strings.HasPrefix(pkg, prefix+"/") {
				violations = append(violations, "package "+pkg)
			}
		}
	}
	for _, edge := range loadProductionEdges(t) {
		for _, prefix := range forbidden {
			if edge.imported == prefix || strings.HasPrefix(edge.imported, prefix+"/") {
				violations = append(violations, "import "+formatEdge(edge))
			}
		}
	}
	if len(violations) != 0 {
		sort.Strings(violations)
		t.Fatalf("state SQL network source prefixes remain:\n%s", strings.Join(violations, "\n"))
	}
}

func TestStateSQLNetworkImplementationImportsStayOutbound(t *testing.T) {
	for _, edge := range loadProductionEdges(t) {
		if !isStateSQLNetworkCapability(edge.importer) || isOutboundAdapter(edge.importer) {
			continue
		}
		if edge.imported == "database/sql" || edge.imported == "os" || edge.imported == "os/exec" || edge.imported == "net/http" || strings.Contains(edge.imported, "sqlite") {
			t.Fatalf("state SQL network implementation escaped outbound adapter: %s", formatEdge(edge))
		}
	}
}

func isStateSQLNetworkCapability(path string) bool {
	for _, prefix := range []string{
		"internal/application/state",
		"internal/application/webfetch",
		"internal/contract/state",
		"internal/contract/webfetch",
		"internal/domain/state",
		"internal/domain/statepath",
		"internal/domain/webfetch",
		"internal/port/state",
		"internal/port/webfetch",
	} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func TestProductionReseedRoutingHasNoLegacyFallback(t *testing.T) {
	violations, err := productionReseedRoutingViolations(findRepoRoot(t))
	if err != nil {
		t.Fatalf("inspect production reseed routing: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("production reseed routing violations: %s", strings.Join(violations, "; "))
	}
}

func TestProductionResumeRoutingHasNoLegacyFallback(t *testing.T) {
	violations, err := productionResumeRoutingViolations(findRepoRoot(t))
	if err != nil {
		t.Fatalf("inspect production resume routing: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("production resume routing violations: %s", strings.Join(violations, "; "))
	}
}

func TestDependencyProductionReconcileRoutingHasNoLegacyFallback(t *testing.T) {
	violations, err := productionReconcileRoutingViolations(findRepoRoot(t))
	if err != nil {
		t.Fatalf("inspect production reconcile routing: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("production reconcile routing violations: %s", strings.Join(violations, "; "))
	}
}

func TestDependencyProductionPublicationCallersHaveNoConcreteProviderResolver(t *testing.T) {
	violations, err := productionPublicationCallerViolations(findRepoRoot(t))
	if err != nil {
		t.Fatalf("inspect production publication callers: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("production publication caller violations: %s", strings.Join(violations, "; "))
	}
}

func TestDependencyProductionPublicationCoreHasNoLegacyOrchestration(t *testing.T) {
	violations, err := productionPublicationLegacyOrchestrationViolations(findRepoRoot(t))
	if err != nil {
		t.Fatalf("inspect production publication orchestration: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("production publication orchestration violations: %s", strings.Join(violations, "; "))
	}
}

func TestDependencyProductionPreparationHasNoLegacyFallbackOrConcreteCaller(t *testing.T) {
	violations, err := productionPreparationRoutingViolations(findRepoRoot(t))
	if err != nil {
		t.Fatalf("inspect production preparation routing: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("production preparation routing violations: %s", strings.Join(violations, "; "))
	}
}

func TestDependencyPreparationRoutingViolationsRejectLegacyFallback(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "router.go", `package issueops
func route(req request, deps dependencies) {
	switch req.Action {
	case ExecutionActionPrepare:
		invokeExecutionPrepareHandler()
		invokeExecutionPrepareHandler()
		PrepareExecution()
	}
}
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	violations := preparationRoutingViolations(file, true)
	joined := strings.Join(violations, "; ")
	if !strings.Contains(joined, "does not invoke injected prepare handler exactly once") || !strings.Contains(joined, "calls legacy PrepareExecution") {
		t.Fatalf("legacy preparation fallback violations=%v", violations)
	}
}

func TestDependencyPreparationCallerViolationsRejectConcreteConstruction(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "caller.go", `package caller
import (
	wt "agent-harness/internal/adapter/gitworktree"
	"agent-harness/internal/adapter/orca"
)
func issueOpsExecutionDeps() { _ = wt.New(); _ = orca.NewExecution() }
func unrelatedCleanup() { _ = orca.NewExecution() }
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	violations := preparationCallerViolations(file, "caller.go")
	if len(violations) != 2 || strings.Contains(strings.Join(violations, "; "), "unrelatedCleanup") {
		t.Fatalf("preparation caller violations=%v", violations)
	}
}

func TestDependencyPublicationLegacyOrchestrationViolationsRejectDefinitionsAndCalls(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "remote.go", `package issueops
func createRemotePullRequestLegacy() {}
func route() { reconcileRemotePullRequest() }
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	violations := publicationLegacyOrchestrationViolations(file, "remote.go")
	joined := strings.Join(violations, "; ")
	if !strings.Contains(joined, "createRemotePullRequestLegacy") || !strings.Contains(joined, "reconcileRemotePullRequest") {
		t.Fatalf("publication legacy orchestration violations=%v", violations)
	}
}

func TestDependencyPublicationCallerViolationsIgnoreUnrelatedRemoteOperations(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "remote.go", `package caller
import "agent-harness/internal/adapter/provider"
func createIssue() { provider.Resolve("github") }
func createPullRequest() { provider.Resolve("github") }
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	violations := publicationCallerViolations(file, "remote.go")
	if len(violations) != 1 || !strings.Contains(violations[0], "createPullRequest") {
		t.Fatalf("publication caller violations=%v", violations)
	}
}

func TestDependencyReconcileRoutingViolationsRejectLegacyFallback(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "router.go", `package issueops
func route(deps dependencies) {
	reconcileOrcaExecutionIntent()
}
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	violations := reconcileRoutingViolations(file, true)
	joined := strings.Join(violations, "; ")
	if !strings.Contains(joined, "does not invoke injected Handler") || !strings.Contains(joined, "calls legacy reconcileOrcaExecutionIntent") {
		t.Fatalf("legacy fallback violations=%v", violations)
	}
}

func TestResumeRoutingViolationsRejectLegacyFallback(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "router.go", `package issueops
func route(req request, deps dependencies) {
	switch req.Action {
	case ExecutionActionResume:
		ResumeExecutionWithDependencies()
	}
}
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	violations := resumeRoutingViolations(file, true)
	if len(violations) == 0 || !strings.Contains(strings.Join(violations, "; "), "does not invoke injected Resume handler") || !strings.Contains(strings.Join(violations, "; "), "calls legacy ResumeExecutionWithDependencies") {
		t.Fatalf("legacy fallback violations=%v", violations)
	}
}

func TestReseedOwnerArtifactPreparationDoesNotReapplyLeaseTransition(t *testing.T) {
	path := filepath.Join(findRepoRoot(t), "internal", "adapter", "issueops", "execution_reseed_adapter.go")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"record.Execution.Lease.Generation++",
		"record.Execution.Lease.Status =",
		"record.Execution.Lease.Holder =",
		"record.Execution.Lease.ClaimTokenSHA256 =",
	} {
		if strings.Contains(string(contents), forbidden) {
			t.Fatalf("owner artifact preparation must use the application transition, found %q", forbidden)
		}
	}
}

func TestProductionReseedWiringUsesOutboundInventory(t *testing.T) {
	path := filepath.Join(findRepoRoot(t), "cmd", "harness", "harnessapp", "issueops_reseed_wiring.go")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "ObserveExecutionReseed") || !strings.Contains(string(contents), "NewReseedInventory") {
		t.Fatal("production reseed wiring must use the outbound inventory adapter")
	}
}

func TestReseedRoutingViolationsRejectLegacyFallback(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "router.go", `package issueops
func route(req request, deps dependencies) {
	if req.ReplaceAction == ExecutionReplaceReseed {
		ReplaceExecutionWithDependencies()
	}
}
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	violations := reseedRoutingViolations(file, true)
	if len(violations) == 0 || !strings.Contains(strings.Join(violations, "; "), "does not invoke injected Reseed handler") || !strings.Contains(strings.Join(violations, "; "), "calls legacy ReplaceExecutionWithDependencies") {
		t.Fatalf("legacy fallback violations=%v", violations)
	}
}

func productionReseedRoutingViolations(repoRoot string) ([]string, error) {
	dir := filepath.Join(repoRoot, "internal", "adapter", "issueops")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var violations []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, contents, 0)
		if err != nil {
			return nil, err
		}
		violations = append(violations, reseedRoutingViolations(file, name == "execution_api.go")...)
	}
	sort.Strings(violations)
	return violations, nil
}

func productionResumeRoutingViolations(repoRoot string) ([]string, error) {
	dir := filepath.Join(repoRoot, "internal", "adapter", "issueops")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var violations []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, contents, 0)
		if err != nil {
			return nil, err
		}
		if sourceHasIdentifier(file, "ResumeExecutionWithDependencies") {
			violations = append(violations, name+" retains legacy resume orchestration")
		}
		violations = append(violations, resumeRoutingViolations(file, name == "execution_api.go")...)
	}
	outboundDir := filepath.Join(repoRoot, "internal", "adapter", "outbound", "issueopslease")
	entries, err = os.ReadDir(outboundDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(outboundDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		for _, forbidden := range []string{"executeOrcaIntentStage", "ResumeExecutionWithDependencies"} {
			if strings.Contains(string(contents), forbidden) {
				violations = append(violations, entry.Name()+" calls "+forbidden)
			}
		}
	}
	sort.Strings(violations)
	return violations, nil
}

func productionReconcileRoutingViolations(repoRoot string) ([]string, error) {
	coreDir := filepath.Join(repoRoot, "internal", "adapter", "issueops")
	entries, err := os.ReadDir(coreDir)
	if err != nil {
		return nil, err
	}
	var violations []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(coreDir, name)
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, contents, 0)
		if err != nil {
			return nil, err
		}
		if sourceHasIdentifier(file, "reconcileOrcaExecutionIntent") {
			violations = append(violations, name+" retains legacy Orca reconcile orchestration")
		}
		if name == "execution_reconcile.go" {
			violations = append(violations, reconcileRoutingViolations(file, true)...)
		}
	}
	wiringViolations, err := reconcileWiringForbiddenCalls(repoRoot)
	if err != nil {
		return nil, err
	}
	violations = append(violations, wiringViolations...)
	sort.Strings(violations)
	return violations, nil
}

func productionPublicationCallerViolations(repoRoot string) ([]string, error) {
	var violations []string
	for _, relative := range []string{filepath.Join("cmd", "harness", "issueopscli"), filepath.Join("cmd", "harness", "mcpcli")} {
		dir := filepath.Join(repoRoot, relative)
		err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() || !strings.HasSuffix(info.Name(), ".go") || strings.HasSuffix(info.Name(), "_test.go") || info.Name() == "issueops_reset_legacy_cli.go" {
				return nil
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, contents, 0)
			if err != nil {
				return err
			}
			name, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return err
			}
			violations = append(violations, publicationCallerViolations(file, filepath.ToSlash(name))...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(violations)
	return violations, nil
}

func productionPreparationRoutingViolations(repoRoot string) ([]string, error) {
	var violations []string
	coreDir := filepath.Join(repoRoot, "internal", "adapter", "issueops")
	entries, err := os.ReadDir(coreDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(coreDir, name)
		file, err := parseProductionFile(path)
		if err != nil {
			return nil, err
		}
		for _, identifier := range []string{"PrepareExecution", "prepareDirectExecution", "prepareOrcaExecution", "ExecutionPrepareDependencies"} {
			if sourceHasIdentifier(file, identifier) {
				violations = append(violations, name+" retains legacy preparation orchestration "+identifier)
			}
		}
		violations = append(violations, preparationRoutingViolations(file, name == "execution_api.go")...)
	}

	for _, relative := range []string{filepath.Join("cmd", "harness", "issueopscli"), filepath.Join("cmd", "harness", "mcpcli")} {
		dir := filepath.Join(repoRoot, relative)
		err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() || !strings.HasSuffix(info.Name(), ".go") || strings.HasSuffix(info.Name(), "_test.go") {
				return nil
			}
			file, err := parseProductionFile(path)
			if err != nil {
				return err
			}
			name, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return err
			}
			violations = append(violations, preparationCallerViolations(file, filepath.ToSlash(name))...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	for _, wiring := range []struct {
		path     string
		function string
	}{
		{path: filepath.Join("cmd", "harness", "harnessapp", "issueops_policy_facade.go"), function: "runIssueOps"},
		{path: filepath.Join("cmd", "harness", "harnessapp", "mcp_facade.go"), function: "issueOpsMCPDependencies"},
	} {
		file, err := parseProductionFile(filepath.Join(repoRoot, wiring.path))
		if err != nil {
			return nil, err
		}
		if count := functionCallCount(file, wiring.function, "productionIssueOpsExecutionDependencies"); count != 1 {
			violations = append(violations, fmt.Sprintf("%s:%s must call the shared execution composition constructor exactly once, found %d", filepath.ToSlash(wiring.path), wiring.function, count))
		}
	}
	sort.Strings(violations)
	return violations, nil
}

func parseProductionFile(path string) (*ast.File, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parser.ParseFile(token.NewFileSet(), path, contents, 0)
}

func preparationRoutingViolations(file *ast.File, requireInjectedHandler bool) []string {
	var violations []string
	routes := 0
	ast.Inspect(file, func(node ast.Node) bool {
		branch, ok := node.(*ast.CaseClause)
		if !ok || !sourceHasIdentifier(branch, "ExecutionActionPrepare") {
			return true
		}
		routes++
		calls := sourceCallCounts(branch)
		if requireInjectedHandler && calls["invokeExecutionPrepareHandler"] != 1 {
			violations = append(violations, "prepare route does not invoke injected prepare handler exactly once")
		}
		for _, legacy := range []string{"PrepareExecution", "prepareDirectExecution", "prepareOrcaExecution"} {
			if calls[legacy] != 0 {
				violations = append(violations, "prepare route calls legacy "+legacy)
			}
		}
		return true
	})
	if requireInjectedHandler && routes != 1 {
		violations = append(violations, fmt.Sprintf("expected one production prepare route, found %d", routes))
	}
	return violations
}

func preparationCallerViolations(file *ast.File, name string) []string {
	aliases := concretePreparationImportAliases(file)
	if len(aliases) == 0 {
		return nil
	}
	var violations []string
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil || !preparationCallerFunction(function) {
			continue
		}
		for alias, imported := range aliases {
			if sourceHasIdentifier(function.Body, alias) {
				violations = append(violations, name+":"+function.Name.Name+" uses concrete "+imported)
			}
		}
	}
	sort.Strings(violations)
	return violations
}

func preparationCallerFunction(function *ast.FuncDecl) bool {
	name := strings.ToLower(function.Name.Name)
	return strings.Contains(name, "preparation") || strings.Contains(name, "prepare") ||
		(strings.Contains(name, "execution") && (strings.Contains(name, "dep") || sourceHasIdentifier(function, "Prepare"))) ||
		sourceHasIdentifier(function, "ExecutionActionPrepare")
}

func concretePreparationImportAliases(file *ast.File) map[string]string {
	aliases := map[string]string{}
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			continue
		}
		if path != "agent-harness/internal/adapter/gitworktree" && path != "agent-harness/internal/adapter/orca" && path != "agent-harness/internal/adapter/provider" {
			continue
		}
		alias := filepath.Base(path)
		if imported.Name != nil {
			alias = imported.Name.Name
		}
		if alias != "_" && alias != "." {
			aliases[alias] = strings.TrimPrefix(path, "agent-harness/")
		}
	}
	return aliases
}

func functionCallCount(file *ast.File, functionName, callName string) int {
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == functionName && function.Body != nil {
			return sourceCallCounts(function.Body)[callName]
		}
	}
	return 0
}

func productionPublicationLegacyOrchestrationViolations(repoRoot string) ([]string, error) {
	coreDir := filepath.Join(repoRoot, "internal", "adapter", "issueops")
	entries, err := os.ReadDir(coreDir)
	if err != nil {
		return nil, err
	}
	var violations []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(coreDir, name)
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, contents, 0)
		if err != nil {
			return nil, err
		}
		violations = append(violations, publicationLegacyOrchestrationViolations(file, name)...)
	}
	sort.Strings(violations)
	return violations, nil
}

func publicationLegacyOrchestrationViolations(file *ast.File, name string) []string {
	var violations []string
	for _, identifier := range []string{"createRemotePullRequestLegacy", "reconcileRemotePullRequest"} {
		if sourceHasIdentifier(file, identifier) {
			violations = append(violations, name+" retains legacy publication orchestration "+identifier)
		}
	}
	return violations
}

func publicationCallerViolations(file *ast.File, name string) []string {
	var violations []string
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil || !publicationCallerFunction(function) || !sourceHasSelectorCall(function.Body, "provider", "Resolve") {
			continue
		}
		violations = append(violations, name+":"+function.Name.Name+" calls provider.Resolve")
	}
	return violations
}

func publicationCallerFunction(function *ast.FuncDecl) bool {
	name := strings.ToLower(function.Name.Name)
	if strings.Contains(name, "publication") || strings.Contains(name, "pullrequest") {
		return true
	}
	for _, identifier := range []string{
		"RemotePullRequestDependencies", "RemotePublicationHandlers", "CreateRemotePullRequest", "ReconcileRemotePullRequest",
	} {
		if sourceHasIdentifier(function, identifier) {
			return true
		}
	}
	return false
}

func sourceHasSelectorCall(node ast.Node, qualifier, name string) bool {
	found := false
	ast.Inspect(node, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != name {
			return true
		}
		identifier, ok := selector.X.(*ast.Ident)
		if ok && identifier.Name == qualifier {
			found = true
			return false
		}
		return true
	})
	return found
}

func reconcileWiringForbiddenCalls(repoRoot string) ([]string, error) {
	var violations []string
	for _, relative := range []string{
		filepath.Join("internal", "adapter", "outbound", "issueopslease"),
		filepath.Join("cmd", "harness", "harnessapp"),
	} {
		dir := filepath.Join(repoRoot, relative)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			contents, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				return nil, err
			}
			for _, forbidden := range []string{"reconcileOrcaExecutionIntent", "executeOrcaIntentStage"} {
				if strings.Contains(string(contents), forbidden) {
					violations = append(violations, entry.Name()+" calls "+forbidden)
				}
			}
		}
	}
	return violations, nil
}

func reseedRoutingViolations(file *ast.File, requireInjectedHandler bool) []string {
	var violations []string
	routes := 0
	ast.Inspect(file, func(node ast.Node) bool {
		branch, ok := node.(*ast.IfStmt)
		if !ok || !sourceHasIdentifier(branch.Cond, "ExecutionReplaceReseed") {
			return true
		}
		routes++
		calls := sourceCallNames(branch.Body)
		if requireInjectedHandler && !calls["Reseed"] {
			violations = append(violations, "reseed route does not invoke injected Reseed handler")
		}
		if calls["ReplaceExecutionWithDependencies"] {
			violations = append(violations, "reseed route calls legacy ReplaceExecutionWithDependencies")
		}
		return true
	})
	if requireInjectedHandler && routes != 1 {
		violations = append(violations, fmt.Sprintf("expected one production reseed route, found %d", routes))
	}
	return violations
}

func resumeRoutingViolations(file *ast.File, requireInjectedHandler bool) []string {
	var violations []string
	routes := 0
	ast.Inspect(file, func(node ast.Node) bool {
		branch, ok := node.(*ast.CaseClause)
		if !ok || !sourceHasIdentifier(branch, "ExecutionActionResume") {
			return true
		}
		routes++
		calls := sourceCallCounts(branch)
		if requireInjectedHandler && calls["Resume"] != 1 {
			violations = append(violations, "resume route does not invoke injected Resume handler exactly once")
		}
		if calls["ResumeExecutionWithDependencies"] != 0 {
			violations = append(violations, "resume route calls legacy ResumeExecutionWithDependencies")
		}
		return true
	})
	if requireInjectedHandler && routes != 1 {
		violations = append(violations, fmt.Sprintf("expected one production resume route, found %d", routes))
	}
	return violations
}

func reconcileRoutingViolations(file *ast.File, requireInjectedHandler bool) []string {
	counts := sourceCallCounts(file)
	var violations []string
	if requireInjectedHandler && counts["Handler"] != 1 {
		violations = append(violations, "reconcile route does not invoke injected Handler exactly once")
	}
	if counts["reconcileOrcaExecutionIntent"] != 0 {
		violations = append(violations, "reconcile route calls legacy reconcileOrcaExecutionIntent")
	}
	return violations
}

func sourceHasIdentifier(node ast.Node, name string) bool {
	found := false
	ast.Inspect(node, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if ok && identifier.Name == name {
			found = true
			return false
		}
		return !found
	})
	return found
}

func sourceCallNames(node ast.Node) map[string]bool {
	calls := map[string]bool{}
	ast.Inspect(node, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch function := call.Fun.(type) {
		case *ast.Ident:
			calls[function.Name] = true
		case *ast.SelectorExpr:
			calls[function.Sel.Name] = true
		}
		return true
	})
	return calls
}

func sourceCallCounts(node ast.Node) map[string]int {
	counts := map[string]int{}
	ast.Inspect(node, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch function := call.Fun.(type) {
		case *ast.Ident:
			counts[function.Name]++
		case *ast.SelectorExpr:
			counts[function.Sel.Name]++
		}
		return true
	})
	return counts
}

func evaluateEdges(edges []dependencyEdge) []violation {
	var violations []violation
	for _, edge := range edges {
		if isCore(edge.importer) && (isAdapter(edge.imported) || isCommand(edge.imported)) && !isWaveTwoStorageAdapterEdge(edge) {
			violations = append(violations, violation{"core_must_not_import_adapter_or_cmd", edge})
		}
		if isAdapter(edge.importer) && isCommand(edge.imported) {
			violations = append(violations, violation{"adapter_must_not_import_cmd", edge})
		}
		// port는 계약 어휘로 말한다. 인터페이스 시그니처가 DTO를 참조하는 것은
		// 구현 의존이 아니라 계약 사용이므로 port -> contract는 허용한다. domain·
		// application·adapter·cmd로 향하는 edge는 그대로 막힌다.
		if isPort(edge.importer) && strings.HasPrefix(edge.imported, "internal/") && !isContract(edge.imported) && !isPort(edge.imported) {
			violations = append(violations, violation{"port_must_not_import_internal", edge})
		}
		if isDomain(edge.importer) && isDomainImplementation(edge.imported) && !isAllowedDomainContract(edge) {
			violations = append(violations, violation{"domain_must_not_import_implementation", edge})
		}
		if isApplication(edge.importer) && isApplicationImplementation(edge.imported) {
			violations = append(violations, violation{"application_must_not_import_implementation", edge})
		}
		if isPublicationContract(edge.importer) && (strings.HasPrefix(edge.imported, "internal/") || edge.imported == "database/sql") {
			violations = append(violations, violation{"publication_contract_must_not_import_internal", edge})
		}
		if isPublicationDomain(edge.importer) && strings.HasPrefix(edge.imported, "internal/") && !isPublicationContract(edge.imported) {
			violations = append(violations, violation{"publication_domain_must_only_import_contract", edge})
		}
		if isPublicationApplication(edge.importer) && strings.HasPrefix(edge.imported, "internal/") && !isPublicationDomain(edge.imported) && !isPublicationContract(edge.imported) {
			violations = append(violations, violation{"publication_application_must_only_import_domain_or_contract", edge})
		}
		if isPublicationOutboundAdapter(edge.importer) && isCore(edge.imported) {
			violations = append(violations, violation{"publication_outbound_adapter_must_not_import_core", edge})
		}
		if isCompletionContract(edge.importer) && strings.HasPrefix(edge.imported, "internal/") && edge.imported != "internal/contract/issueopslease" {
			violations = append(violations, violation{"completion_contract_must_only_import_stable_contract", edge})
		}
		if isCompletionDomain(edge.importer) && strings.HasPrefix(edge.imported, "internal/") && !isCompletionContract(edge.imported) {
			violations = append(violations, violation{"completion_domain_must_only_import_contract", edge})
		}
		if isCompletionApplication(edge.importer) && strings.HasPrefix(edge.imported, "internal/") && !isCompletionDomain(edge.imported) && !isCompletionContract(edge.imported) {
			violations = append(violations, violation{"completion_application_must_only_import_domain_or_contract", edge})
		}
		if isCompletionOutboundAdapter(edge.importer) && isCore(edge.imported) {
			violations = append(violations, violation{"completion_outbound_adapter_must_not_import_core", edge})
		}
		if isPreparationContract(edge.importer) && strings.HasPrefix(edge.imported, "internal/") && edge.imported != "internal/contract/issueopslease" {
			violations = append(violations, violation{"preparation_contract_must_only_import_lease_contract", edge})
		}
		if isPreparationDomain(edge.importer) && strings.HasPrefix(edge.imported, "internal/") && !isPreparationContract(edge.imported) && !isLeaseContract(edge.imported) {
			violations = append(violations, violation{"preparation_domain_must_only_import_contract", edge})
		}
		if isPreparationApplication(edge.importer) && strings.HasPrefix(edge.imported, "internal/") && !isPreparationDomain(edge.imported) && !isPreparationContract(edge.imported) && !isLeaseContract(edge.imported) {
			violations = append(violations, violation{"preparation_application_must_only_import_domain_or_contract", edge})
		}
		if isPreparationOutboundAdapter(edge.importer) && isCore(edge.imported) {
			violations = append(violations, violation{"preparation_outbound_adapter_must_not_import_core", edge})
		}
		if isLeaseVerticalLayer(edge.importer, "contract") && isProductionIssueOps(edge.imported) {
			violations = append(violations, violation{"leasevertical_contract_must_not_import_production_issueops", edge})
		}
		if isInboundAdapter(edge.importer) && isOutboundAdapter(edge.imported) {
			violations = append(violations, violation{"inbound_adapter_must_not_import_outbound_adapter", edge})
		}
	}
	return violations
}

func isWaveTwoStorageAdapterEdge(edge dependencyEdge) bool {
	if !isCore(edge.importer) {
		return false
	}
	// T6가 storage 구현을 먼저 이동하고 T7이 남은 core caller를 다음 merge에서
	// 제거하므로, 봉인된 두 adapter만 중간 parent HEAD에서 허용한다.
	return edge.imported == "internal/adapter/outbound/sqlstore" || edge.imported == "internal/adapter/outbound/state"
}

func evaluateOwnershipEdges(edges []dependencyEdge) []violation {
	var violations []violation
	for _, edge := range edges {
		if isCore(edge.importer) || isCore(edge.imported) {
			violations = append(violations, violation{"ownership_forbids_core_package", edge})
			continue
		}
		if isContract(edge.importer) && strings.HasPrefix(edge.imported, "internal/") && !isContract(edge.imported) {
			violations = append(violations, violation{"contract_must_not_import_internal", edge})
		}
		if isDomain(edge.importer) && strings.HasPrefix(edge.imported, "internal/") && !isContract(edge.imported) {
			violations = append(violations, violation{"domain_must_only_import_contract", edge})
		}
		if isApplication(edge.importer) && (isAdapter(edge.imported) || isCommand(edge.imported)) {
			violations = append(violations, violation{"application_must_not_import_adapter_or_cmd", edge})
		}
		// port는 계약 어휘로 말한다. 인터페이스 시그니처가 DTO를 참조하는 것은
		// 구현 의존이 아니라 계약 사용이므로 port -> contract는 허용한다. domain·
		// application·adapter·cmd로 향하는 edge는 그대로 막힌다.
		if isPort(edge.importer) && strings.HasPrefix(edge.imported, "internal/") && !isContract(edge.imported) && !isPort(edge.imported) {
			violations = append(violations, violation{"port_must_not_import_internal", edge})
		}
	}
	return violations
}

func foundationOwnershipViolations(edges []dependencyEdge) []violation {
	var violations []violation
	for _, edge := range edges {
		if isCoreIssueOpsModel(edge.importer) || isCoreIssueOpsModel(edge.imported) {
			violations = append(violations, violation{"ownership_forbids_core_package", edge})
			continue
		}
		if isFoundationOwner(edge.importer) {
			violations = append(violations, evaluateOwnershipEdges([]dependencyEdge{edge})...)
		}
	}
	return violations
}

func foundationPackageViolations(packages []string) []string {
	var violations []string
	for _, path := range packages {
		if isCoreIssueOpsModel(path) {
			violations = append(violations, "ownership_forbids_core_package: "+path)
		}
	}
	sort.Strings(violations)
	return violations
}

func isCoreIssueOpsModel(path string) bool {
	const prefix = "internal/core/issueops/model"
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func isFoundationOwner(path string) bool {
	for _, prefix := range []string{
		"internal/application/nativeactivation",
		"internal/contract/issueops",
		"internal/contract/lifecycle",
		"internal/contract/nativeactivation",
		"internal/contract/state",
		"internal/domain/issueops",
		"internal/domain/lifecycle",
		"internal/domain/policy",
		"internal/domain/state",
		"internal/port/nativeactivation",
		"internal/port/policy",
		"internal/port/state",
	} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func isPublicationDomainContract(edge dependencyEdge) bool {
	return edge.importer == "internal/domain/issueopspublication" && edge.imported == "internal/contract/issueopspublication"
}

func isAllowedDomainContract(edge dependencyEdge) bool {
	return isSameCapabilityContract(edge) || isPublicationDomainContract(edge) ||
		edge.importer == "internal/domain/issueopscompletion" && edge.imported == "internal/contract/issueopscompletion" ||
		edge.importer == "internal/domain/issueopspreparation" && edge.imported == "internal/contract/issueopspreparation"
}

func isSameCapabilityContract(edge dependencyEdge) bool {
	const domainPrefix = "internal/domain/"
	const contractPrefix = "internal/contract/"
	if !strings.HasPrefix(edge.importer, domainPrefix) || !strings.HasPrefix(edge.imported, contractPrefix) {
		return false
	}
	return strings.TrimPrefix(edge.importer, domainPrefix) == strings.TrimPrefix(edge.imported, contractPrefix)
}

func isPublicationContract(path string) bool {
	return path == "internal/contract/issueopspublication" || strings.HasPrefix(path, "internal/contract/issueopspublication/")
}

func isPublicationDomain(path string) bool {
	return path == "internal/domain/issueopspublication" || strings.HasPrefix(path, "internal/domain/issueopspublication/")
}

func isPublicationApplication(path string) bool {
	return path == "internal/application/issueopspublication" || strings.HasPrefix(path, "internal/application/issueopspublication/")
}

func isPublicationOutboundAdapter(path string) bool {
	return path == "internal/adapter/outbound/issueopspublication" || strings.HasPrefix(path, "internal/adapter/outbound/issueopspublication/")
}

func isCompletionContract(path string) bool {
	return path == "internal/contract/issueopscompletion" || strings.HasPrefix(path, "internal/contract/issueopscompletion/")
}

func isCompletionDomain(path string) bool {
	return path == "internal/domain/issueopscompletion" || strings.HasPrefix(path, "internal/domain/issueopscompletion/")
}

func isCompletionApplication(path string) bool {
	return path == "internal/application/issueopscompletion" || strings.HasPrefix(path, "internal/application/issueopscompletion/")
}

func isCompletionOutboundAdapter(path string) bool {
	return path == "internal/adapter/outbound/issueopscompletion" || strings.HasPrefix(path, "internal/adapter/outbound/issueopscompletion/")
}

func isPreparationContract(path string) bool {
	return path == "internal/contract/issueopspreparation" || strings.HasPrefix(path, "internal/contract/issueopspreparation/")
}

func isLeaseContract(path string) bool {
	return path == "internal/contract/issueopslease" || strings.HasPrefix(path, "internal/contract/issueopslease/")
}

func isPreparationDomain(path string) bool {
	return path == "internal/domain/issueopspreparation" || strings.HasPrefix(path, "internal/domain/issueopspreparation/")
}

func isPreparationApplication(path string) bool {
	return path == "internal/application/issueopspreparation" || strings.HasPrefix(path, "internal/application/issueopspreparation/")
}

func isPreparationOutboundAdapter(path string) bool {
	return path == "internal/adapter/outbound/issueopspreparation" || strings.HasPrefix(path, "internal/adapter/outbound/issueopspreparation/")
}

func containsViolation(violations []violation, rule string, edge dependencyEdge) bool {
	for _, got := range violations {
		if got.rule == rule && got.edge == edge {
			return true
		}
	}
	return false
}

func loadProductionEdges(t *testing.T) []dependencyEdge {
	t.Helper()
	repoRoot := findRepoRoot(t)
	command := exec.Command("go", "list", "-json", "./...")
	command.Dir = repoRoot
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go list -json ./...: %v", err)
	}

	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var edges []dependencyEdge
	for decoder.More() {
		var pkg struct {
			ImportPath string
			Imports    []string
		}
		if err := decoder.Decode(&pkg); err != nil {
			t.Fatalf("decode go list package: %v", err)
		}
		if !strings.HasPrefix(pkg.ImportPath, "agent-harness/") {
			continue
		}
		for _, imported := range pkg.Imports {
			edges = append(edges, dependencyEdge{normalizeImport(pkg.ImportPath), normalizeImport(imported)})
		}
	}
	return sortedEdges(edges)
}

func loadProductionPackages(t *testing.T) []string {
	t.Helper()
	repoRoot := findRepoRoot(t)
	command := exec.Command("go", "list", "-json", "./...")
	command.Dir = repoRoot
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go list -json ./...: %v", err)
	}

	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var packages []string
	for decoder.More() {
		var pkg struct {
			ImportPath string
		}
		if err := decoder.Decode(&pkg); err != nil {
			t.Fatalf("decode go list package: %v", err)
		}
		if strings.HasPrefix(pkg.ImportPath, "agent-harness/") {
			packages = append(packages, normalizeImport(pkg.ImportPath))
		}
	}
	sort.Strings(packages)
	return packages
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found from test directory")
		}
		dir = parent
	}
}

func legacyEdges(edges []dependencyEdge) []dependencyEdge {
	var legacy []dependencyEdge
	for _, edge := range edges {
		if (isCore(edge.importer) && isLegacyInfrastructure(edge.imported)) ||
			(isAdapter(edge.importer) && isCore(edge.imported) && !isMigratedInboundAdapter(edge.importer)) ||
			(isConcreteAdapter(edge.imported) && !isCompositionRoot(edge.importer) && !isSameCapabilityAdapter(edge.importer, edge.imported) &&
				!isSharedStorageEngineEdge(edge.importer, edge.imported)) {
			legacy = append(legacy, edge)
		}
	}
	return sortedEdges(legacy)
}

func normalizeImport(path string) string {
	return strings.TrimPrefix(path, "agent-harness/")
}

func sortedEdges(edges []dependencyEdge) []dependencyEdge {
	unique := make(map[dependencyEdge]struct{}, len(edges))
	for _, edge := range edges {
		unique[edge] = struct{}{}
	}
	sorted := make([]dependencyEdge, 0, len(unique))
	for edge := range unique {
		sorted = append(sorted, edge)
	}
	sort.Slice(sorted, func(i, j int) bool { return formatEdge(sorted[i]) < formatEdge(sorted[j]) })
	return sorted
}

func difference(left, right []dependencyEdge) []dependencyEdge {
	set := make(map[dependencyEdge]struct{}, len(right))
	for _, edge := range right {
		set[edge] = struct{}{}
	}
	var diff []dependencyEdge
	for _, edge := range left {
		if _, found := set[edge]; !found {
			diff = append(diff, edge)
		}
	}
	return diff
}

func formatViolations(violations []violation) string {
	lines := make([]string, 0, len(violations))
	for _, got := range violations {
		lines = append(lines, got.rule+": "+formatEdge(got.edge))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func formatEdge(edge dependencyEdge) string {
	return edge.importer + " -> " + edge.imported
}

func isCore(path string) bool {
	return path == "internal/core" || strings.HasPrefix(path, "internal/core/")
}

func isAdapter(path string) bool {
	return path == "internal/adapter" || strings.HasPrefix(path, "internal/adapter/") || isLeaseVerticalLayer(path, "adapter")
}

func isPort(path string) bool {
	return path == "internal/port" || strings.HasPrefix(path, "internal/port/")
}

func isCommand(path string) bool { return path == "cmd" || strings.HasPrefix(path, "cmd/") }

func isDomain(path string) bool {
	return path == "internal/domain" || strings.HasPrefix(path, "internal/domain/") || isLeaseVerticalLayer(path, "domain")
}

func isApplication(path string) bool {
	return path == "internal/application" || strings.HasPrefix(path, "internal/application/") || isLeaseVerticalLayer(path, "application")
}

func isConcreteAdapter(path string) bool { return isAdapter(path) }

// adapterCapability는 adapter 경로가 구현하는 capability 이름을 돌려준다.
// outbound/inbound는 capability가 아니라 방향 분류이므로 그 다음 요소까지 읽는다.
func adapterCapability(path string) string {
	rest := strings.TrimPrefix(path, "internal/adapter/")
	if rest == path {
		return ""
	}
	parts := strings.Split(rest, "/")
	if (parts[0] == "outbound" || parts[0] == "inbound") && len(parts) > 1 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

// isSameCapabilityAdapter는 두 adapter가 같은 capability의 구현인지 판정한다.
//
// 하나의 adapter를 하위 package로 나누는 것은 계층 위반이 아니라 구현 정리다.
// 이를 adapter 간 결합으로 세면 package를 잘게 나눌수록 벌점이 되어, 커다란
// package를 유지할 유인이 생긴다. capability 경계를 넘는 의존만 legacy로 센다.
func isSameCapabilityAdapter(importer, imported string) bool {
	if !isAdapter(importer) || !isAdapter(imported) {
		return false
	}
	capability := adapterCapability(importer)
	return capability != "" && capability == adapterCapability(imported)
}

// outbound/sqlstore는 특정 capability의 어댑터가 아니라 저장 엔진 자체다.
// state와 issueops는 각자의 레코드를 같은 sqlite 파일에 담으며, 엔진을 포트로
// 감싸 주입으로 갈아끼우는 것은 가능하지만 그 대가로 이 저장소의 거의 모든
// 테스트 패키지가 배선을 짊어지게 된다. 엔진 교체는 실제 요구가 아니므로,
// outbound 어댑터가 공유 저장 엔진을 직접 쓰는 것은 의도된 설계로 못박는다.
// 허용 범위는 outbound -> sqlstore 한 방향뿐이고, cmd·inbound·domain에서
// 들어오는 edge는 그대로 막힌다.
func isSharedStorageEngineEdge(importer, imported string) bool {
	if imported != "internal/adapter/outbound/sqlstore" {
		return false
	}
	return strings.HasPrefix(importer, "internal/adapter/outbound/") ||
		importer == "internal/adapter/issueops"
}

func isInboundAdapter(path string) bool {
	return path == "internal/adapter/inbound" || strings.HasPrefix(path, "internal/adapter/inbound/")
}

func isOutboundAdapter(path string) bool {
	return path == "internal/adapter/outbound" || strings.HasPrefix(path, "internal/adapter/outbound/")
}

func isCompositionRoot(path string) bool { return path == "cmd/harness/harnessapp" }

func isDomainImplementation(path string) bool {
	return isApplication(path) || isAdapter(path) || isCommand(path) || isContract(path) || path == "os" || path == "os/exec" || path == "net" || path == "net/http" || path == "database/sql" || path == "syscall" || strings.Contains(path, "sqlite")
}

func isContract(path string) bool {
	return path == "internal/contract" || strings.HasPrefix(path, "internal/contract/") || isLeaseVerticalLayer(path, "contract")
}

func isApplicationImplementation(path string) bool {
	return isAdapter(path) || isCommand(path) || path == "os" || path == "os/exec" || path == "path/filepath" || path == "net" || path == "net/http" || path == "database/sql" || path == "syscall" || strings.Contains(path, "sqlite")
}

func isLeaseVerticalLayer(path, layer string) bool {
	return path == "internal/"+layer+"/issueopslease"
}

func isMigratedInboundAdapter(path string) bool {
	return path == "internal/adapter/inbound/issueopslease" || path == "internal/adapter/inbound/issueopspublication" || path == "internal/adapter/inbound/issueopscompletion" || path == "internal/adapter/inbound/issueopspreparation"
}

func isProductionIssueOps(path string) bool {
	return path == "internal/core/issueops" ||
		(strings.HasPrefix(path, "internal/core/issueops/") && !strings.HasPrefix(path, "internal/core/issueops/testdata/"))
}

func isLegacyInfrastructure(path string) bool {
	return path == "os" || path == "os/exec" || path == "net" || path == "net/http" || path == "database/sql" || path == "syscall" || strings.Contains(path, "sqlite")
}
