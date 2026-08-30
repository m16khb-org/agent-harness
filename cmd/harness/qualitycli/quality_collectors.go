package qualitycli

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"agent-harness/cmd/harness/selfworkflow"
	"agent-harness/internal/domain/qualitycatalog"
)

func collectSelfAugmentOpenCount(root string) (int, error) {
	oldRoot := selfworkflow.HarnessRoot
	oldVersion := selfworkflow.Version
	selfworkflow.HarnessRoot = func() string { return root }
	selfworkflow.Version = hostDeps.Version
	defer func() {
		selfworkflow.HarnessRoot = oldRoot
		selfworkflow.Version = oldVersion
	}()
	plan := selfworkflow.PlanSelfAugmentation(selfworkflow.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95})
	return len(selfworkflow.SelfAugmentCandidateIDsByStatus(plan.Candidates, selfworkflow.SelfAugmentCandidateStatusOpen)), nil
}

func collectSelfVerifyOpenCount(root string) (int, error) {
	oldRoot := selfworkflow.HarnessRoot
	selfworkflow.HarnessRoot = func() string { return root }
	defer func() { selfworkflow.HarnessRoot = oldRoot }()
	result := selfworkflow.ExportSelfVerificationCandidates()
	return len(selfworkflow.SelfVerificationCandidateIDsByStatus(result.Candidates, selfworkflow.SelfAugmentCandidateStatusOpen)), nil
}

func collectQualityCandidates(root string) []QualityCandidate {
	candidates := qualitycatalog.Candidates()
	oldRoot := selfworkflow.HarnessRoot
	oldVersion := selfworkflow.Version
	selfworkflow.HarnessRoot = func() string { return root }
	selfworkflow.Version = hostDeps.Version
	defer func() {
		selfworkflow.HarnessRoot = oldRoot
		selfworkflow.Version = oldVersion
	}()

	plan := selfworkflow.PlanSelfAugmentation(selfworkflow.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95})
	statusByID := map[string]selfworkflow.SelfAugmentCandidate{}
	for _, candidate := range plan.Candidates {
		statusByID[candidate.ID] = candidate
	}
	for i := range candidates {
		if projected, ok := statusByID[candidates[i].ID]; ok {
			candidates[i].Status = projected.Status
			candidates[i].Score = projected.Score
		}
	}
	return candidates
}

func parseCoveragePackages(output string, threshold float64) []CoveragePackage {
	var packages []CoveragePackage
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "coverage:") || !strings.Contains(line, "% of statements") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		coverageIdx := -1
		for i, field := range fields {
			if field == "coverage:" {
				coverageIdx = i
				break
			}
		}
		if coverageIdx < 0 || coverageIdx+1 >= len(fields) {
			continue
		}
		valueText := strings.TrimSuffix(fields[coverageIdx+1], "%")
		value, err := strconv.ParseFloat(valueText, 64)
		if err != nil || value >= threshold {
			continue
		}
		packageName := fields[0]
		if (fields[0] == "ok" || fields[0] == "?") && len(fields) > 1 {
			packageName = fields[1]
		}
		packages = append(packages, CoveragePackage{Package: packageName, Coverage: value})
	}
	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Coverage != packages[j].Coverage {
			return packages[i].Coverage < packages[j].Coverage
		}
		return packages[i].Package < packages[j].Package
	})
	return packages
}

func renderCoveragePackages(packages []CoveragePackage) string {
	var output strings.Builder
	for _, item := range packages {
		_, _ = fmt.Fprintf(
			&output,
			"%s coverage: %.1f%% of statements\n",
			item.Package,
			item.Coverage,
		)
	}
	return output.String()
}

func collectBranchFunctions(root string) ([]BranchFunction, []string) {
	functions := []BranchFunction{}
	warnings := []string{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			warnings = append(warnings, "branch scan: "+err.Error())
			return nil
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".codegraph", ".harness", "bin", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			warnings = append(warnings, "branch scan "+path+": "+err.Error())
			return nil
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			branches := countBranches(fn.Body)
			pos := fset.Position(fn.Pos())
			functions = append(functions, BranchFunction{
				File:     relOrAbs(root, path),
				Line:     pos.Line,
				Name:     fn.Name.Name,
				Branches: branches,
			})
		}
		return nil
	})
	if err != nil {
		warnings = append(warnings, "branch scan: "+err.Error())
	}
	sort.Slice(functions, func(i, j int) bool {
		if functions[i].Branches != functions[j].Branches {
			return functions[i].Branches > functions[j].Branches
		}
		if functions[i].File != functions[j].File {
			return functions[i].File < functions[j].File
		}
		return functions[i].Line < functions[j].Line
	})
	return functions, warnings
}

func countBranches(node ast.Node) int {
	branches := 0
	ast.Inspect(node, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.CaseClause, *ast.CommClause:
			branches++
		}
		return true
	})
	return branches
}

func collectAuditItems(root string) ([]AuditItem, []string) {
	path := filepath.Join(root, ".agent-harness", "PROJECT_AUDIT.md")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []string{"audit scan: " + err.Error()}
	}
	items := []AuditItem{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || strings.Contains(line, "---") {
			continue
		}
		parts := splitMarkdownRow(line)
		if len(parts) < 5 || parts[0] == "ID" {
			continue
		}
		priority := strings.TrimSpace(parts[3])
		if priority != "P0" && priority != "P1" && priority != "P2" {
			continue
		}
		items = append(items, AuditItem{
			ID:       strings.TrimSpace(parts[0]),
			Area:     strings.TrimSpace(parts[1]),
			Title:    strings.TrimSpace(parts[2]),
			Priority: priority,
			Size:     strings.TrimSpace(parts[4]),
		})
	}
	return items, nil
}

func splitMarkdownRow(line string) []string {
	line = strings.Trim(line, "|")
	raw := strings.Split(line, "|")
	out := make([]string, 0, len(raw))
	for _, part := range raw {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

func statusForCount(count int) string {
	if count > 0 {
		return "needs_attention"
	}
	return "ok"
}

func statusForCollector(err error, fallback string) string {
	if err != nil {
		return "error"
	}
	return fallback
}

func pioneerIsolatedStatus(coverage PioneerCoverage) string {
	if coverage.IsolatedExpected == 0 {
		return "unknown"
	}
	if coverage.IsolatedObserved != coverage.IsolatedExpected ||
		coverage.IsolatedFailed > 0 ||
		coverage.IsolatedBlocked > 0 {
		return "needs_attention"
	}
	return "ok"
}

func pioneerIsolatedEvidence(coverage PioneerCoverage) []string {
	return []string{
		fmt.Sprintf(
			"observed=%d expected=%d pass=%d blocked=%d fail=%d hidden=%d",
			coverage.IsolatedObserved,
			coverage.IsolatedExpected,
			coverage.IsolatedPassed,
			coverage.IsolatedBlocked,
			coverage.IsolatedFailed,
			coverage.HiddenHoldoutObserved,
		),
		"fresh-context fixtures are committed reproduction inputs, not hidden holdouts",
	}
}

func firstQualityWarning(warnings []string) error {
	if len(warnings) == 0 {
		return nil
	}
	return errors.New(warnings[0])
}

func coverageEvidence(packages []CoveragePackage) []string {
	evidence := []string{}
	for _, pkg := range packages {
		evidence = append(evidence, fmt.Sprintf("%s %.1f%%", pkg.Package, pkg.Coverage))
	}
	if len(evidence) == 0 {
		return []string{"go test -cover ./..."}
	}
	return evidence
}

func branchEvidence(functions []BranchFunction, threshold int) []string {
	evidence := []string{}
	for _, fn := range functions {
		if fn.Branches <= threshold {
			continue
		}
		evidence = append(evidence, fmt.Sprintf("%s:%d %s branches=%d", fn.File, fn.Line, fn.Name, fn.Branches))
		if len(evidence) == 10 {
			break
		}
	}
	if len(evidence) == 0 {
		return []string{"Go AST branch scan"}
	}
	return evidence
}

func auditEvidence(items []AuditItem) []string {
	evidence := []string{}
	for _, item := range items {
		evidence = append(evidence, fmt.Sprintf("%s %s %s", item.ID, item.Priority, item.Title))
	}
	if len(evidence) == 0 {
		return []string{".agent-harness/PROJECT_AUDIT.md"}
	}
	return evidence
}

func relOrAbs(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}
