package qualitycli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	type textResult struct {
		value string
		err   error
	}
	type branchResult struct {
		value    []BranchFunction
		warnings []string
	}
	type auditResult struct {
		value    []AuditItem
		warnings []string
	}
	coverageResults := make(chan textResult, 1)
	branchResults := make(chan branchResult, 1)
	auditResults := make(chan auditResult, 1)
	snrResults := make(chan SNRResult, 1)
	go func() {
		value, err := deps.Coverage(root)
		coverageResults <- textResult{value: value, err: err}
	}()
	go func() {
		value, warnings := collectBranchFunctions(root)
		branchResults <- branchResult{value: value, warnings: warnings}
	}()
	go func() {
		value, warnings := collectAuditItems(root)
		auditResults <- auditResult{value: value, warnings: warnings}
	}()
	go func() {
		snrResults <- deps.CodeSNR(root)
	}()

	selfAugmentOpen, selfAugmentErr := deps.SelfAugmentOpenCount(root)
	selfVerifyOpen, selfVerifyErr := deps.SelfVerifyOpenCount(root)
	candidates := deps.Candidates(root)

	coverage := <-coverageResults
	branches := <-branchResults
	audit := <-auditResults
	snr := <-snrResults
	warnings := []string{}
	if coverage.err != nil {
		warnings = append(warnings, "coverage: "+coverage.err.Error())
	}
	lowCoverage := parseCoveragePackages(coverage.value, 60)
	branchFunctions := branches.value
	warnings = append(warnings, branches.warnings...)
	auditItems := audit.value
	warnings = append(warnings, audit.warnings...)
	if selfAugmentErr != nil {
		warnings = append(warnings, "self-augment candidates: "+selfAugmentErr.Error())
	}
	if selfVerifyErr != nil {
		warnings = append(warnings, "self-verify candidates: "+selfVerifyErr.Error())
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
	fingerprint, fingerprintErr := coverageFingerprint(root)
	if fingerprintErr == nil {
		if output, ok := readCoverageCache(root, fingerprint); ok {
			return output, nil
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	output, err := executeGoTestCoverage(ctx, root)
	if err == nil && fingerprintErr == nil {
		writeCoverageCache(root, fingerprint, output)
	}
	return output, err
}

var executeGoTestCoverage = func(ctx context.Context, root string) (string, error) {
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

const coverageCacheVersion = 1

type coverageCacheEntry struct {
	Version     int    `json:"version"`
	Fingerprint string `json:"fingerprint"`
	Output      string `json:"output"`
}

var coverageCacheBase = func() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "agent-harness", "quality-coverage"), nil
}

func coverageFingerprint(root string) (string, error) {
	head, err := gitCoverageFingerprintInput(root, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return "", err
	}
	diff, err := gitCoverageFingerprintInput(
		root,
		"diff",
		"--binary",
		"--no-ext-diff",
		"--no-textconv",
		"HEAD",
		"--",
	)
	if err != nil {
		return "", err
	}
	untracked, err := gitCoverageFingerprintInput(root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return "", err
	}
	hasher := sha256.New()
	_, _ = fmt.Fprintf(
		hasher,
		"coverage-v%d\ngo=%s\ngoos=%s\ngoarch=%s\n",
		coverageCacheVersion,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
	)
	environment := coverageEnvironmentFingerprint()
	sort.Strings(environment)
	for _, value := range environment {
		_, _ = fmt.Fprintf(hasher, "env:%s\n", value)
	}
	_, _ = hasher.Write(head)
	_, _ = hasher.Write(diff)
	paths := bytes.Split(untracked, []byte{0})
	sort.Slice(paths, func(left, right int) bool {
		return bytes.Compare(paths[left], paths[right]) < 0
	})
	for _, rawPath := range paths {
		if len(rawPath) == 0 {
			continue
		}
		relative := filepath.Clean(string(rawPath))
		if relative == "." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("coverage fingerprint path escapes repository: %s", relative)
		}
		path := filepath.Join(root, relative)
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintf(hasher, "path:%s\nmode:%s\n", filepath.ToSlash(relative), info.Mode())
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return "", err
			}
			_, _ = hasher.Write([]byte(target))
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		_, _ = hasher.Write(data)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func gitCoverageFingerprintInput(root string, args ...string) ([]byte, error) {
	command := exec.Command("git", args...)
	command.Dir = root
	return command.Output()
}

func coverageEnvironmentFingerprint() []string {
	result := []string{}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "GO") ||
			name == "CGO_ENABLED" ||
			name == "CC" ||
			name == "CXX" ||
			name == "PATH" {
			result = append(result, entry)
		}
	}
	return result
}

func coverageCachePath(root string) (string, error) {
	base, err := coverageCacheBase()
	if err != nil {
		return "", err
	}
	rootHash := sha256.Sum256([]byte(filepath.Clean(root)))
	return filepath.Join(base, hex.EncodeToString(rootHash[:16])+".json"), nil
}

func readCoverageCache(root, fingerprint string) (string, bool) {
	path, err := coverageCachePath(root)
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var entry coverageCacheEntry
	if json.Unmarshal(data, &entry) != nil ||
		entry.Version != coverageCacheVersion ||
		entry.Fingerprint != fingerprint {
		return "", false
	}
	return entry.Output, true
}

func writeCoverageCache(root, fingerprint, output string) {
	path, err := coverageCachePath(root)
	if err != nil || os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	data, err := json.Marshal(coverageCacheEntry{
		Version: coverageCacheVersion, Fingerprint: fingerprint, Output: output,
	})
	if err != nil {
		return
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".coverage-*.tmp")
	if err != nil {
		return
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return
	}
	if err := temporary.Close(); err != nil {
		_ = temporary.Close()
		return
	}
	_ = os.Rename(temporaryPath, path)
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
