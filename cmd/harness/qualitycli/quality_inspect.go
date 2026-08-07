package qualitycli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"agent-harness/cmd/harness/selfworkflow"
	"agent-harness/internal/domain/qualitycatalog"
)

type InspectDeps struct {
	Now                  func() string
	Coverage             func(root string) (string, error)
	SelfAugmentOpenCount func(root string) (int, error)
	SelfVerifyOpenCount  func(root string) (int, error)
	Candidates           func(root string) []QualityCandidate
	CodeSNR              func(root string) SNRResult
}

type QualityCandidate = qualitycatalog.Candidate

type InspectResult struct {
	OK          bool               `json:"ok"`
	GeneratedAt string             `json:"generated_at"`
	HarnessRoot string             `json:"harness_root"`
	Summary     Summary            `json:"summary"`
	Signals     []Signal           `json:"signals"`
	Candidates  []QualityCandidate `json:"candidates"`
	Warnings    []string           `json:"warnings"`
}

type Summary struct {
	SelfAugmentOpenCandidates int `json:"self_augment_open_candidates"`
	SelfVerifyOpenCandidates  int `json:"self_verify_open_candidates"`
	LowCoveragePackages       int `json:"low_coverage_packages"`
	BranchCandidateFunctions  int `json:"branch_candidate_functions"`
	HighBranchFunctions       int `json:"high_branch_functions"`
	AuditP1P2Items            int `json:"audit_p1_p2_items"`
	CandidateCount            int `json:"candidate_count"`
}

type Signal struct {
	ID        string   `json:"id"`
	Category  string   `json:"category"`
	Status    string   `json:"status"`
	Value     float64  `json:"value"`
	Threshold float64  `json:"threshold,omitempty"`
	Evidence  []string `json:"evidence"`
}

type CoveragePackage struct {
	Package  string  `json:"package"`
	Coverage float64 `json:"coverage"`
}

type BranchFunction struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Name     string `json:"name"`
	Branches int    `json:"branches"`
}

type AuditItem struct {
	ID       string `json:"id"`
	Area     string `json:"area"`
	Title    string `json:"title"`
	Priority string `json:"priority"`
	Size     string `json:"size"`
}

func Run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: quality inspect [--repo PATH] [--json]")
	}
	switch args[0] {
	case "inspect":
		return RunInspectWithDeps(args[1:], InspectDeps{})
	case "help", "--help", "-h":
		fmt.Println("agent-harness quality inspect [--repo PATH] [--json]")
		return nil
	default:
		return fmt.Errorf("unknown quality command %q", args[0])
	}
}

func RunInspectWithDeps(args []string, deps InspectDeps) error {
	fs := flag.NewFlagSet("quality inspect", flag.ContinueOnError)
	repo := fs.String("repo", hostDeps.HarnessRoot(), "target repository path")
	jsonOut := fs.Bool("json", false, "print JSON")
	saveBaseline := fs.Bool("save-baseline", false, "persist the current code-SNR as the trend baseline in harness state")
	trend := fs.Bool("trend", false, "report the code-SNR delta versus the saved baseline")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*repo = fs.Arg(0)
	}
	result := Inspect(*repo, deps)
	snrRatio := signalValue(result.Signals, "code-snr")
	if *saveBaseline {
		if err := saveSNRBaseline(snrRatio); err != nil {
			result.Warnings = append(result.Warnings, "save-baseline: "+err.Error())
		}
	}
	if *jsonOut {
		return hostDeps.PrintJSON(result)
	}
	fmt.Printf("quality inspect: ok=%v repo=%s candidates=%d warnings=%d\n", result.OK, result.HarnessRoot, len(result.Candidates), len(result.Warnings))
	fmt.Printf("self-augment open: %d\n", result.Summary.SelfAugmentOpenCandidates)
	fmt.Printf("self-verify open: %d\n", result.Summary.SelfVerifyOpenCandidates)
	fmt.Printf("low coverage packages: %d\n", result.Summary.LowCoveragePackages)
	fmt.Printf("branch candidate functions: %d\n", result.Summary.BranchCandidateFunctions)
	fmt.Printf("audit P1/P2 items: %d\n", result.Summary.AuditP1P2Items)
	if *trend {
		if base, ok := readSNRBaseline(); ok {
			fmt.Printf("code-snr: %.4f (baseline %.4f, Δ %+.4f)\n", snrRatio, base, snrRatio-base)
		} else {
			fmt.Printf("code-snr: %.4f (no baseline saved; run with --save-baseline)\n", snrRatio)
		}
	} else {
		fmt.Printf("code-snr: %.4f\n", snrRatio)
	}
	for _, warning := range result.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	return nil
}

// signalValue returns the Value of the signal with the given id, or 0.
func signalValue(signals []Signal, id string) float64 {
	for _, s := range signals {
		if s.ID == id {
			return s.Value
		}
	}
	return 0
}

func Inspect(root string, deps InspectDeps) InspectResult {
	root = resolveRoot(root)
	deps = deps.withDefaults()
	warnings := []string{}

	coverageOutput, err := deps.Coverage(root)
	if err != nil {
		warnings = append(warnings, "coverage: "+err.Error())
	}
	lowCoverage := parseCoveragePackages(coverageOutput, 60)
	branchFunctions, branchWarnings := collectBranchFunctions(root)
	warnings = append(warnings, branchWarnings...)
	auditItems, auditWarnings := collectAuditItems(root)
	warnings = append(warnings, auditWarnings...)

	selfAugmentOpen, err := deps.SelfAugmentOpenCount(root)
	if err != nil {
		warnings = append(warnings, "self-augment candidates: "+err.Error())
	}
	selfVerifyOpen, err := deps.SelfVerifyOpenCount(root)
	if err != nil {
		warnings = append(warnings, "self-verify candidates: "+err.Error())
	}
	highBranchCount := 0
	branchCandidateCount := 0
	for _, fn := range branchFunctions {
		if fn.Branches > 6 {
			branchCandidateCount++
		}
		if fn.Branches > 12 {
			highBranchCount++
		}
	}
	candidates := deps.Candidates(root)
	snr := deps.CodeSNR(root)
	signals := []Signal{
		{ID: "self-augment-open-candidates", Category: "candidate", Status: "ok", Value: float64(selfAugmentOpen), Evidence: []string{"self-augment candidate catalog"}},
		{ID: "self-verify-open-candidates", Category: "candidate", Status: "ok", Value: float64(selfVerifyOpen), Evidence: []string{"self-verify candidate export"}},
		{ID: "low-coverage-packages", Category: "coverage", Status: statusForCount(len(lowCoverage)), Value: float64(len(lowCoverage)), Threshold: 60, Evidence: coverageEvidence(lowCoverage)},
		{ID: "branch-candidate-functions", Category: "complexity", Status: statusForCount(branchCandidateCount), Value: float64(branchCandidateCount), Threshold: 6, Evidence: branchEvidence(branchFunctions, 6)},
		{ID: "high-branch-functions", Category: "complexity", Status: statusForCount(highBranchCount), Value: float64(highBranchCount), Threshold: 12, Evidence: branchEvidence(branchFunctions, 12)},
		{ID: "audit-p1-p2-items", Category: "audit", Status: statusForCount(len(auditItems)), Value: float64(len(auditItems)), Evidence: auditEvidence(auditItems)},
		{ID: "code-snr", Category: "quality", Status: "ok", Value: snr.Ratio, Evidence: snrEvidence(snr)},
	}
	return InspectResult{
		OK:          true,
		GeneratedAt: deps.Now(),
		HarnessRoot: root,
		Summary: Summary{
			SelfAugmentOpenCandidates: selfAugmentOpen,
			SelfVerifyOpenCandidates:  selfVerifyOpen,
			LowCoveragePackages:       len(lowCoverage),
			BranchCandidateFunctions:  branchCandidateCount,
			HighBranchFunctions:       highBranchCount,
			AuditP1P2Items:            len(auditItems),
			CandidateCount:            len(candidates),
		},
		Signals:    signals,
		Candidates: candidates,
		Warnings:   warnings,
	}
}

func (deps InspectDeps) withDefaults() InspectDeps {
	if deps.Now == nil {
		deps.Now = func() string { return time.Now().UTC().Format(time.RFC3339Nano) }
	}
	if deps.Coverage == nil {
		deps.Coverage = runGoTestCoverage
	}
	if deps.SelfAugmentOpenCount == nil {
		deps.SelfAugmentOpenCount = collectSelfAugmentOpenCount
	}
	if deps.SelfVerifyOpenCount == nil {
		deps.SelfVerifyOpenCount = collectSelfVerifyOpenCount
	}
	if deps.Candidates == nil {
		deps.Candidates = collectQualityCandidates
	}
	if deps.CodeSNR == nil {
		deps.CodeSNR = computeCodeSNR
	}
	return deps
}

func resolveRoot(root string) string {
	if root == "" {
		root = hostDeps.HarnessRoot()
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	return abs
}

func runGoTestCoverage(root string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-cover", "./...")
	cmd.Dir = root
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return out.String(), ctx.Err()
	}
	return out.String(), err
}

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
		if suppressLowCoveragePackage(fields[1]) {
			continue
		}
		packages = append(packages, CoveragePackage{Package: fields[1], Coverage: value})
	}
	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Coverage != packages[j].Coverage {
			return packages[i].Coverage < packages[j].Coverage
		}
		return packages[i].Package < packages[j].Package
	})
	return packages
}

func suppressLowCoveragePackage(pkg string) bool {
	return pkg == "agent-harness/internal/adapter/core" || strings.HasSuffix(pkg, "/internal/core")
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
		if priority != "P1" && priority != "P2" {
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
