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

func TestCompareBaselineRejectsNewAndStaleEdges(t *testing.T) {
	newEdge := dependencyEdge{"internal/adapter/cli", "internal/core/issueops"}
	staleEdge := dependencyEdge{"internal/core/issueops", "os/exec"}

	if err := compareBaseline([]dependencyEdge{newEdge}, nil); err == nil || !strings.Contains(err.Error(), "legacy_baseline: new legacy edge: internal/adapter/cli -> internal/core/issueops") {
		t.Fatalf("expected exact new-edge error, got %v", err)
	}
	if err := compareBaseline(nil, []dependencyEdge{staleEdge}); err == nil || !strings.Contains(err.Error(), "legacy_baseline: stale legacy edge: internal/core/issueops -> os/exec") {
		t.Fatalf("expected exact stale-edge error, got %v", err)
	}
}

func TestParseBaselineRejectsUnsortedAndDuplicateEdges(t *testing.T) {
	if got, err := parseBaseline(""); err != nil || len(got) != 0 {
		t.Fatalf("expected empty baseline to be valid, got %v, %v", got, err)
	}
	if _, err := parseBaseline(`internal/core/z -> os
internal/core/a -> os`); err == nil || !strings.Contains(err.Error(), "legacy_baseline: baseline is not sorted") {
		t.Fatalf("expected sorted-baseline error, got %v", err)
	}
	if _, err := parseBaseline(`internal/core/a -> os
internal/core/a -> os`); err == nil || !strings.Contains(err.Error(), "legacy_baseline: duplicate edge: internal/core/a -> os") {
		t.Fatalf("expected duplicate-baseline error, got %v", err)
	}
}

func TestProductionGraphMatchesBaseline(t *testing.T) {
	edges := loadProductionEdges(t)
	if got := evaluateEdges(edges); len(got) != 0 {
		t.Fatalf("forbidden dependency violations:\n%s", formatViolations(got))
	}

	secondInventory := loadProductionEdges(t)
	if !reflect.DeepEqual(edges, secondInventory) {
		t.Fatalf("production import inventory is not byte-stable")
	}

	baseline := readBaseline(t)
	if err := compareBaseline(legacyEdges(edges), baseline); err != nil {
		t.Fatal(err)
	}
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
	path := filepath.Join(findRepoRoot(t), "internal", "core", "issueops", "execution_reseed_adapter.go")
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
	dir := filepath.Join(repoRoot, "internal", "core", "issueops")
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
		if sourceHasIdentifier(file, "reseedExecutionCompatibilityOracle") {
			violations = append(violations, name+" retains the legacy reseed oracle")
		}
		violations = append(violations, reseedRoutingViolations(file, name == "execution_api.go")...)
	}
	sort.Strings(violations)
	return violations, nil
}

func productionResumeRoutingViolations(repoRoot string) ([]string, error) {
	dir := filepath.Join(repoRoot, "internal", "core", "issueops")
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
		if calls["reseedExecutionCompatibilityOracle"] {
			violations = append(violations, "reseed route calls the legacy reseed oracle")
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
		if isCore(edge.importer) && (isAdapter(edge.imported) || isCommand(edge.imported)) {
			violations = append(violations, violation{"core_must_not_import_adapter_or_cmd", edge})
		}
		if isAdapter(edge.importer) && isCommand(edge.imported) {
			violations = append(violations, violation{"adapter_must_not_import_cmd", edge})
		}
		if isPort(edge.importer) && strings.HasPrefix(edge.imported, "internal/") {
			violations = append(violations, violation{"port_must_not_import_internal", edge})
		}
		if isDomain(edge.importer) && isDomainImplementation(edge.imported) {
			violations = append(violations, violation{"domain_must_not_import_implementation", edge})
		}
		if isApplication(edge.importer) && isApplicationImplementation(edge.imported) {
			violations = append(violations, violation{"application_must_not_import_implementation", edge})
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

func compareBaseline(observed, baseline []dependencyEdge) error {
	observed = sortedEdges(observed)
	baseline = sortedEdges(baseline)
	var problems []string
	for _, edge := range difference(observed, baseline) {
		problems = append(problems, "legacy_baseline: new legacy edge: "+formatEdge(edge))
	}
	for _, edge := range difference(baseline, observed) {
		problems = append(problems, "legacy_baseline: stale legacy edge: "+formatEdge(edge))
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(problems, "\n"))
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

func readBaseline(t *testing.T) []dependencyEdge {
	t.Helper()
	contents, err := os.ReadFile("testdata/legacy_imports.txt")
	if err != nil {
		t.Fatalf("read legacy baseline: %v", err)
	}
	edges, err := parseBaseline(string(contents))
	if err != nil {
		t.Fatal(err)
	}
	return edges
}

func parseBaseline(contents string) ([]dependencyEdge, error) {
	if strings.TrimSpace(contents) == "" {
		return nil, nil
	}
	var edges []dependencyEdge
	previous := ""
	for _, line := range strings.Split(strings.TrimSpace(contents), "\n") {
		parts := strings.Split(line, " -> ")
		if len(parts) != 2 {
			return nil, fmt.Errorf("legacy_baseline: invalid edge: %q", line)
		}
		edge := dependencyEdge{parts[0], parts[1]}
		current := formatEdge(edge)
		if current == previous {
			return nil, fmt.Errorf("legacy_baseline: duplicate edge: %s", current)
		}
		if previous != "" && current < previous {
			return nil, fmt.Errorf("legacy_baseline: baseline is not sorted: %s before %s", current, previous)
		}
		previous = current
		edges = append(edges, edge)
	}
	return edges, nil
}

func legacyEdges(edges []dependencyEdge) []dependencyEdge {
	var legacy []dependencyEdge
	for _, edge := range edges {
		if (isCore(edge.importer) && isLegacyInfrastructure(edge.imported)) ||
			(isAdapter(edge.importer) && isCore(edge.imported) && !isReleaseInboundAdapter(edge.importer)) ||
			(isConcreteAdapter(edge.imported) && !isCompositionRoot(edge.importer)) {
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

func isReleaseInboundAdapter(path string) bool {
	return path == "internal/adapter/inbound/issueopslease"
}

func isProductionIssueOps(path string) bool {
	return path == "internal/core/issueops" ||
		(strings.HasPrefix(path, "internal/core/issueops/") && !strings.HasPrefix(path, "internal/core/issueops/testdata/"))
}

func isLegacyInfrastructure(path string) bool {
	return path == "os" || path == "os/exec" || path == "net" || path == "net/http" || path == "database/sql" || path == "syscall" || strings.Contains(path, "sqlite")
}
