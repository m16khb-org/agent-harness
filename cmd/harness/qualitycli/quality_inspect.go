package qualitycli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/domain/pioneerskill"
	"agent-harness/internal/domain/qualitycatalog"
)

type InspectDeps struct {
	Now                  func() string
	Coverage             func(root string) (string, error)
	SelfAugmentOpenCount func(root string) (int, error)
	SelfVerifyOpenCount  func(root string) (int, error)
	Candidates           func(root string) []QualityCandidate
	CodeSNR              func(root string) (SNRResult, error)
	PioneerCoverage      func(root string) (PioneerCoverage, error)
	SaveSNRBaseline      func(string, float64) error
	ReadSNRBaseline      func(string) (float64, bool, error)
}

var ErrQualityGateBlocked = errors.New("quality gate blocked")

type QualityCandidate = qualitycatalog.Candidate

const (
	CollectionStatusOK    = "ok"
	CollectionStatusError = "error"

	HealthStatusHealthy        = "healthy"
	HealthStatusNeedsAttention = "needs_attention"
	HealthStatusUnknown        = "unknown"

	GateStatusPass       = "pass"
	GateStatusReportOnly = "report_only"
	GateStatusBlock      = "block"
)

type InspectResult struct {
	OK               bool               `json:"ok"`
	CollectionStatus string             `json:"collection_status"`
	HealthStatus     string             `json:"health_status"`
	GateStatus       string             `json:"gate_status"`
	GeneratedAt      string             `json:"generated_at"`
	HarnessRoot      string             `json:"harness_root"`
	Summary          Summary            `json:"summary"`
	Signals          []Signal           `json:"signals"`
	Findings         []Finding          `json:"findings"`
	PioneerCoverage  PioneerCoverage    `json:"pioneer_coverage"`
	Candidates       []QualityCandidate `json:"candidates"`
	Warnings         []string           `json:"warnings"`
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

type Finding struct {
	ID            string   `json:"id"`
	Severity      string   `json:"severity"`
	Title         string   `json:"title"`
	Blocking      bool     `json:"blocking"`
	Evidence      []string `json:"evidence"`
	Remediation   string   `json:"remediation"`
	VerifyCommand string   `json:"verify_command"`
}

type PioneerCoverage struct {
	Expected               int                  `json:"expected"`
	BenchmarkObserved      int                  `json:"benchmark_observed"`
	BenchmarkMissing       []string             `json:"benchmark_missing"`
	ReproductionObserved   int                  `json:"reproduction_observed"`
	ReproductionMissing    []string             `json:"reproduction_missing"`
	IsolatedExpected       int                  `json:"isolated_expected"`
	IsolatedObserved       int                  `json:"isolated_observed"`
	IsolatedPassed         int                  `json:"isolated_passed"`
	IsolatedBlocked        int                  `json:"isolated_blocked"`
	IsolatedFailed         int                  `json:"isolated_failed"`
	IsolatedExecutionCount int                  `json:"isolated_execution_count"`
	IsolatedBlockedCases   []PioneerBlockedCase `json:"isolated_blocked_cases"`
	HiddenHoldoutObserved  int                  `json:"hidden_holdout_observed"`
}

type PioneerBlockedCase struct {
	Skill  string `json:"skill"`
	Axis   string `json:"axis"`
	Reason string `json:"reason"`
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
	deps = deps.withDefaults()
	result := Inspect(*repo, deps)
	snrRatio, snrAvailable := successfulSignalValue(result.Signals, "code-snr")
	baseline, baselinePresent := float64(0), false
	if *trend {
		var baselineErr error
		baseline, baselinePresent, baselineErr = deps.ReadSNRBaseline(*repo)
		if baselineErr != nil {
			addQualityCollectorFailure(&result, "read-baseline: "+baselineErr.Error())
		}
	}
	if *saveBaseline {
		if !snrAvailable {
			addQualityCollectorFailure(&result, "save-baseline: code-snr signal is unavailable")
		} else if err := deps.SaveSNRBaseline(*repo, snrRatio); err != nil {
			addQualityCollectorFailure(&result, "save-baseline: "+err.Error())
		}
	}
	if *trend && snrAvailable && baselinePresent && snrRatio < baseline-0.01 {
		addSNRRegressionFinding(&result, baseline, snrRatio)
	}
	gateErr := error(nil)
	if result.GateStatus == GateStatusBlock {
		gateErr = fmt.Errorf("%w: collection=%s health=%s", ErrQualityGateBlocked, result.CollectionStatus, result.HealthStatus)
	}
	if *jsonOut {
		if err := hostDeps.PrintJSON(result); err != nil {
			return err
		}
		return gateErr
	}
	fmt.Printf("quality inspect: ok=%v repo=%s candidates=%d warnings=%d\n", result.OK, result.HarnessRoot, len(result.Candidates), len(result.Warnings))
	fmt.Printf("quality status: collection=%s health=%s gate=%s\n", result.CollectionStatus, result.HealthStatus, result.GateStatus)
	fmt.Printf("self-augment open: %d\n", result.Summary.SelfAugmentOpenCandidates)
	fmt.Printf("self-verify open: %d\n", result.Summary.SelfVerifyOpenCandidates)
	fmt.Printf("low coverage packages: %d\n", result.Summary.LowCoveragePackages)
	fmt.Printf("branch candidate functions: %d\n", result.Summary.BranchCandidateFunctions)
	fmt.Printf(
		"pioneer coverage: benchmark=%d/%d reproduction=%d/%d\n",
		result.PioneerCoverage.BenchmarkObserved,
		result.PioneerCoverage.Expected,
		result.PioneerCoverage.ReproductionObserved,
		result.PioneerCoverage.Expected,
	)
	fmt.Printf("audit P0/P1/P2 items: %d\n", result.Summary.AuditP1P2Items)
	if *trend {
		if baselinePresent {
			fmt.Printf("code-snr: %.4f (baseline %.4f, Δ %+.4f)\n", snrRatio, baseline, snrRatio-baseline)
		} else {
			fmt.Printf("code-snr: %.4f (no baseline saved; run with --save-baseline)\n", snrRatio)
		}
	} else {
		fmt.Printf("code-snr: %.4f\n", snrRatio)
	}
	for _, warning := range result.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	return gateErr
}

func addQualityCollectorFailure(result *InspectResult, warning string) {
	result.Warnings = append(result.Warnings, warning)
	found := false
	for index := range result.Findings {
		if result.Findings[index].ID == "quality-collector-error" {
			result.Findings[index].Evidence = append(result.Findings[index].Evidence, warning)
			found = true
			break
		}
	}
	if !found {
		result.Findings = append([]Finding{{
			ID:            "quality-collector-error",
			Severity:      "p0",
			Title:         "Quality evidence collection failed",
			Blocking:      true,
			Evidence:      []string{warning},
			Remediation:   "Repair the failing collector before relying on repository health.",
			VerifyCommand: "./bin/agent-harness quality inspect --json",
		}}, result.Findings...)
	}
	result.OK = false
	result.CollectionStatus = CollectionStatusError
	result.HealthStatus = HealthStatusUnknown
	result.GateStatus = GateStatusBlock
}

func addSNRRegressionFinding(result *InspectResult, baseline, current float64) {
	result.Findings = append(result.Findings, Finding{
		ID:            "code-snr-regression",
		Severity:      "p1",
		Title:         "Code signal-to-noise regressed from baseline",
		Blocking:      true,
		Evidence:      []string{fmt.Sprintf("baseline=%.4f current=%.4f delta=%+.4f", baseline, current, current-baseline)},
		Remediation:   "Inspect the changed production code for avoidable structural noise or explicitly save an approved new baseline.",
		VerifyCommand: "./bin/agent-harness quality inspect --trend --json",
	})
	result.HealthStatus = HealthStatusNeedsAttention
	result.GateStatus = GateStatusBlock
}

func successfulSignalValue(signals []Signal, id string) (float64, bool) {
	for _, signal := range signals {
		if signal.ID == id {
			return signal.Value, signal.Status == "ok"
		}
	}
	return 0, false
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
	type pioneerResult struct {
		value PioneerCoverage
		err   error
	}
	type snrResult struct {
		value SNRResult
		err   error
	}
	coverageResults := make(chan textResult, 1)
	branchResults := make(chan branchResult, 1)
	auditResults := make(chan auditResult, 1)
	snrResults := make(chan snrResult, 1)
	pioneerResults := make(chan pioneerResult, 1)
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
		value, err := deps.CodeSNR(root)
		snrResults <- snrResult{value: value, err: err}
	}()
	go func() {
		value, err := deps.PioneerCoverage(root)
		pioneerResults <- pioneerResult{value: value, err: err}
	}()

	selfAugmentOpen, selfAugmentErr := deps.SelfAugmentOpenCount(root)
	selfVerifyOpen, selfVerifyErr := deps.SelfVerifyOpenCount(root)
	candidates := deps.Candidates(root)

	coverage := <-coverageResults
	branches := <-branchResults
	audit := <-auditResults
	snr := <-snrResults
	pioneer := <-pioneerResults
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
	if pioneer.err != nil {
		warnings = append(warnings, "pioneer coverage: "+pioneer.err.Error())
	}
	if snr.err != nil {
		warnings = append(warnings, "code-snr: "+snr.err.Error())
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
		{ID: "self-augment-open-candidates", Category: "candidate", Status: statusForCollector(selfAugmentErr, "ok"), Value: float64(selfAugmentOpen), Evidence: []string{"self-augment candidate catalog"}},
		{ID: "self-verify-open-candidates", Category: "candidate", Status: statusForCollector(selfVerifyErr, "ok"), Value: float64(selfVerifyOpen), Evidence: []string{"self-verify candidate export"}},
		{ID: "low-coverage-packages", Category: "coverage", Status: statusForCollector(coverage.err, statusForCount(len(lowCoverage))), Value: float64(len(lowCoverage)), Threshold: 60, Evidence: coverageEvidence(lowCoverage)},
		{ID: "branch-candidate-functions", Category: "complexity", Status: statusForCollector(firstQualityWarning(branches.warnings), statusForCount(branchCandidateCount)), Value: float64(branchCandidateCount), Threshold: 6, Evidence: branchEvidence(branchFunctions, 6)},
		{ID: "high-branch-functions", Category: "complexity", Status: statusForCount(highBranchCount), Value: float64(highBranchCount), Threshold: 12, Evidence: branchEvidence(branchFunctions, 12)},
		{ID: "audit-p0-p1-p2-items", Category: "audit", Status: statusForCollector(firstQualityWarning(audit.warnings), statusForCount(len(auditItems))), Value: float64(len(auditItems)), Evidence: auditEvidence(auditItems)},
		{ID: "pioneer-benchmark-coverage", Category: "skill", Status: statusForCollector(pioneer.err, statusForCount(len(pioneer.value.BenchmarkMissing))), Value: float64(pioneer.value.BenchmarkObserved), Threshold: float64(pioneer.value.Expected), Evidence: append([]string(nil), pioneer.value.BenchmarkMissing...)},
		{ID: "pioneer-reproduction-coverage", Category: "skill", Status: statusForCollector(pioneer.err, statusForCount(len(pioneer.value.ReproductionMissing))), Value: float64(pioneer.value.ReproductionObserved), Threshold: float64(pioneer.value.Expected), Evidence: append([]string(nil), pioneer.value.ReproductionMissing...)},
		{ID: "pioneer-isolated-evaluation", Category: "skill", Status: statusForCollector(pioneer.err, pioneerIsolatedStatus(pioneer.value)), Value: float64(pioneer.value.IsolatedObserved), Threshold: float64(pioneer.value.IsolatedExpected), Evidence: pioneerIsolatedEvidence(pioneer.value)},
		{ID: "code-snr", Category: "quality", Status: statusForCollector(snr.err, "ok"), Value: snr.value.Ratio, Evidence: snrEvidence(snr.value)},
	}
	findings := collectQualityFindings(warnings, lowCoverage, branchFunctions, auditItems, pioneer.value)
	collectionStatus, healthStatus, gateStatus := qualityStatuses(warnings, findings)
	return InspectResult{
		OK:               collectionStatus == CollectionStatusOK,
		CollectionStatus: collectionStatus,
		HealthStatus:     healthStatus,
		GateStatus:       gateStatus,
		GeneratedAt:      deps.Now(),
		HarnessRoot:      root,
		Summary: Summary{
			SelfAugmentOpenCandidates: selfAugmentOpen,
			SelfVerifyOpenCandidates:  selfVerifyOpen,
			LowCoveragePackages:       len(lowCoverage),
			BranchCandidateFunctions:  branchCandidateCount,
			HighBranchFunctions:       highBranchCount,
			AuditP1P2Items:            len(auditItems),
			CandidateCount:            len(candidates),
		},
		Signals:         signals,
		Findings:        findings,
		PioneerCoverage: pioneer.value,
		Candidates:      candidates,
		Warnings:        warnings,
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
	if deps.PioneerCoverage == nil {
		deps.PioneerCoverage = collectPioneerCoverage
	}
	if deps.SaveSNRBaseline == nil {
		deps.SaveSNRBaseline = saveSNRBaseline
	}
	if deps.ReadSNRBaseline == nil {
		deps.ReadSNRBaseline = readSNRBaseline
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

func collectPioneerCoverage(root string) (PioneerCoverage, error) {
	names := pioneerskill.Names()
	benchmarkObserved := make([]string, 0, len(names))
	reproductionObserved := make([]string, 0, len(names))
	for _, name := range names {
		benchmarkPath := filepath.Join(root, "testdata", "issueops", "fixtures", "pioneer-"+name+".json")
		exists, err := regularFileExists(benchmarkPath)
		if err != nil {
			return PioneerCoverage{}, err
		}
		if exists {
			benchmarkObserved = append(benchmarkObserved, name)
		}
		reproductionPath := filepath.Join(root, "testdata", "pioneer-holdouts", name, "TASK.md")
		exists, err = regularFileExists(reproductionPath)
		if err != nil {
			return PioneerCoverage{}, err
		}
		if exists {
			reproductionObserved = append(reproductionObserved, name)
		}
	}
	isolated, err := collectPioneerEvaluationManifest(root, names)
	if err != nil {
		return PioneerCoverage{}, err
	}
	return PioneerCoverage{
		Expected:               len(names),
		BenchmarkObserved:      len(benchmarkObserved),
		BenchmarkMissing:       pioneerskill.Missing(benchmarkObserved),
		ReproductionObserved:   len(reproductionObserved),
		ReproductionMissing:    pioneerskill.Missing(reproductionObserved),
		IsolatedExpected:       len(names) * 3,
		IsolatedObserved:       isolated.observed,
		IsolatedPassed:         isolated.passed,
		IsolatedBlocked:        isolated.blocked,
		IsolatedFailed:         isolated.failed,
		IsolatedExecutionCount: isolated.executions,
		IsolatedBlockedCases:   append([]PioneerBlockedCase(nil), isolated.blockedCases...),
		HiddenHoldoutObserved:  isolated.hidden,
	}, nil
}

type pioneerEvaluationCounts struct {
	observed     int
	passed       int
	blocked      int
	failed       int
	hidden       int
	executions   int
	blockedCases []PioneerBlockedCase
}

func collectPioneerEvaluationManifest(root string, names []string) (pioneerEvaluationCounts, error) {
	path := filepath.Join(root, "testdata", "pioneer-holdouts", "evaluation-manifest.json")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return pioneerEvaluationCounts{}, nil
	}
	if err != nil {
		return pioneerEvaluationCounts{}, err
	}
	var manifest struct {
		SchemaVersion int `json:"schema_version"`
		Provenance    struct {
			Host                    string `json:"host"`
			ExecutionCount          int    `json:"execution_count"`
			CaseCount               int    `json:"case_count"`
			ReceiptAlgorithm        string `json:"receipt_algorithm"`
			ReceiptSource           string `json:"receipt_source"`
			AnswersCommitted        bool   `json:"answers_committed"`
			HiddenHoldouts          bool   `json:"hidden_holdouts"`
			EvidenceRecordCount     int    `json:"evidence_record_count"`
			EvidenceRecordAlgorithm string `json:"evidence_record_algorithm"`
			SemanticGrading         string `json:"semantic_grading"`
		} `json:"provenance"`
		Runs []struct {
			TaskID          string   `json:"task_id"`
			Axes            []string `json:"axes"`
			Status          string   `json:"status"`
			Host            string   `json:"host"`
			Model           string   `json:"model"`
			ReceiptSHA256   string   `json:"receipt_sha256"`
			ReceiptBytes    int      `json:"receipt_bytes"`
			ExecutionMethod string   `json:"execution_method"`
			ArtifactKind    string   `json:"artifact_kind"`
			EvidencePath    string   `json:"evidence_path"`
			EvidenceSHA256  string   `json:"evidence_sha256"`
		} `json:"runs"`
		Cases []struct {
			Skill                   string   `json:"skill"`
			Axis                    string   `json:"axis"`
			CasePath                string   `json:"case_path"`
			CaseSHA256              string   `json:"case_sha256"`
			TaskID                  string   `json:"task_id"`
			Verdict                 string   `json:"verdict"`
			BlockedReason           string   `json:"blocked_reason"`
			HiddenHoldout           bool     `json:"hidden_holdout"`
			EvidencePath            string   `json:"evidence_path"`
			EvidenceSHA256          string   `json:"evidence_sha256"`
			DeterministicAssertions []string `json:"deterministic_assertions"`
			SemanticGrade           string   `json:"semantic_grade"`
			HostCapability          string   `json:"host_capability"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return pioneerEvaluationCounts{}, fmt.Errorf("decode pioneer evaluation manifest: %w", err)
	}
	if manifest.SchemaVersion != 2 {
		return pioneerEvaluationCounts{}, fmt.Errorf("unsupported pioneer evaluation manifest schema %d", manifest.SchemaVersion)
	}
	if strings.TrimSpace(manifest.Provenance.Host) == "" ||
		manifest.Provenance.ExecutionCount != len(manifest.Runs) ||
		manifest.Provenance.CaseCount != len(manifest.Cases) ||
		manifest.Provenance.ReceiptAlgorithm != "sha256" ||
		strings.TrimSpace(manifest.Provenance.ReceiptSource) == "" ||
		manifest.Provenance.AnswersCommitted ||
		manifest.Provenance.HiddenHoldouts ||
		manifest.Provenance.EvidenceRecordCount != len(names) ||
		manifest.Provenance.EvidenceRecordAlgorithm != "sha256" ||
		strings.TrimSpace(manifest.Provenance.SemanticGrading) == "" {
		return pioneerEvaluationCounts{}, fmt.Errorf("invalid pioneer evaluation provenance")
	}
	runAxes := make(map[string]map[string]bool, len(manifest.Runs))
	type evidenceReference struct{ path, digest string }
	runEvidence := make(map[string]evidenceReference, len(manifest.Runs))
	for _, run := range manifest.Runs {
		taskID := strings.TrimSpace(run.TaskID)
		digest, digestErr := hex.DecodeString(run.ReceiptSHA256)
		evidenceDigest, evidenceDigestErr := hex.DecodeString(run.EvidenceSHA256)
		if taskID == "" ||
			runAxes[taskID] != nil ||
			run.Status != "completed" ||
			strings.TrimSpace(run.Host) == "" ||
			strings.TrimSpace(run.Model) == "" ||
			digestErr != nil ||
			len(digest) != sha256.Size ||
			run.ReceiptBytes <= 0 ||
			run.ReceiptBytes > 1<<20 ||
			run.ExecutionMethod != "fresh_context_child_task" ||
			run.ArtifactKind != "bounded_final_response_receipt" ||
			!strings.HasPrefix(run.EvidencePath, "evidence-records/") ||
			evidenceDigestErr != nil ||
			len(evidenceDigest) != sha256.Size {
			return pioneerEvaluationCounts{}, fmt.Errorf("invalid pioneer evaluation run %q", taskID)
		}
		axes := make(map[string]bool, len(run.Axes))
		for _, axis := range run.Axes {
			if axis != "primary" && axis != "boundary" && axis != "operational" {
				return pioneerEvaluationCounts{}, fmt.Errorf("invalid pioneer evaluation run axis %q", axis)
			}
			if axes[axis] {
				return pioneerEvaluationCounts{}, fmt.Errorf("duplicate pioneer evaluation run axis %q", axis)
			}
			axes[axis] = true
		}
		if len(axes) == 0 {
			return pioneerEvaluationCounts{}, fmt.Errorf("pioneer evaluation run %q has no axes", taskID)
		}
		runAxes[taskID] = axes
		runEvidence[taskID] = evidenceReference{path: run.EvidencePath, digest: run.EvidenceSHA256}
	}
	expected := make(map[string]bool, len(names))
	for _, name := range names {
		expected[name] = true
	}
	seen := make(map[string]bool, len(manifest.Cases))
	referencedRunAxes := make(map[string]map[string]bool, len(manifest.Runs))
	var counts pioneerEvaluationCounts
	for _, item := range manifest.Cases {
		expectedFilename := map[string]string{
			"primary":     "TASK.md",
			"boundary":    "BOUNDARY.md",
			"operational": "OPERATIONAL.md",
		}[item.Axis]
		expectedPath := filepath.ToSlash(filepath.Join(item.Skill, expectedFilename))
		expectedEvidencePath := filepath.ToSlash(filepath.Join("evidence-records", item.Skill+".json"))
		key := item.Skill + "/" + item.Axis
		if !expected[item.Skill] ||
			expectedFilename == "" ||
			item.CasePath != expectedPath ||
			seen[key] ||
			strings.TrimSpace(item.TaskID) == "" ||
			!runAxes[item.TaskID][item.Axis] ||
			item.EvidencePath != expectedEvidencePath ||
			runEvidence[item.TaskID] != (evidenceReference{path: item.EvidencePath, digest: item.EvidenceSHA256}) ||
			len(item.DeterministicAssertions) < 2 ||
			strings.TrimSpace(item.SemanticGrade) == "" ||
			strings.TrimSpace(item.HostCapability) == "" {
			return pioneerEvaluationCounts{}, fmt.Errorf("invalid pioneer evaluation case %q", key)
		}
		seen[key] = true
		if referencedRunAxes[item.TaskID] == nil {
			referencedRunAxes[item.TaskID] = map[string]bool{}
		}
		referencedRunAxes[item.TaskID][item.Axis] = true
		task, err := os.ReadFile(filepath.Join(root, "testdata", "pioneer-holdouts", item.CasePath))
		if err != nil {
			return pioneerEvaluationCounts{}, err
		}
		digest := sha256.Sum256(task)
		if fmt.Sprintf("%x", digest[:]) != item.CaseSHA256 {
			return pioneerEvaluationCounts{}, fmt.Errorf("pioneer evaluation case hash mismatch for %s", key)
		}
		evidencePath := filepath.Join(root, "testdata", "pioneer-holdouts", item.EvidencePath)
		info, err := os.Lstat(evidencePath)
		if err != nil || !info.Mode().IsRegular() {
			return pioneerEvaluationCounts{}, fmt.Errorf("invalid pioneer evidence record for %s", key)
		}
		evidenceRaw, err := os.ReadFile(evidencePath)
		if err != nil {
			return pioneerEvaluationCounts{}, err
		}
		evidenceDigest := sha256.Sum256(evidenceRaw)
		if fmt.Sprintf("%x", evidenceDigest[:]) != item.EvidenceSHA256 {
			return pioneerEvaluationCounts{}, fmt.Errorf("pioneer evidence record hash mismatch for %s", key)
		}
		counts.observed++
		if item.HiddenHoldout {
			counts.hidden++
		}
		switch item.Verdict {
		case "pass":
			counts.passed++
		case "blocked":
			if strings.TrimSpace(item.BlockedReason) == "" {
				return pioneerEvaluationCounts{}, fmt.Errorf("blocked pioneer evaluation %s requires blocked_reason", item.Skill)
			}
			counts.blocked++
			counts.blockedCases = append(counts.blockedCases, PioneerBlockedCase{
				Skill: item.Skill, Axis: item.Axis, Reason: item.BlockedReason,
			})
		case "fail":
			counts.failed++
		default:
			return pioneerEvaluationCounts{}, fmt.Errorf("invalid pioneer evaluation verdict %q", item.Verdict)
		}
	}
	for taskID, axes := range runAxes {
		if len(referencedRunAxes[taskID]) != len(axes) {
			return pioneerEvaluationCounts{}, fmt.Errorf("pioneer evaluation run %q axis receipt mismatch", taskID)
		}
		for axis := range axes {
			if !referencedRunAxes[taskID][axis] {
				return pioneerEvaluationCounts{}, fmt.Errorf("pioneer evaluation run %q missing case axis %q", taskID, axis)
			}
		}
	}
	counts.executions = len(manifest.Runs)
	return counts, nil
}

func regularFileExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err == nil {
		return info.Mode().IsRegular(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func collectQualityFindings(
	warnings []string,
	lowCoverage []CoveragePackage,
	branchFunctions []BranchFunction,
	auditItems []AuditItem,
	pioneer PioneerCoverage,
) []Finding {
	findings := make([]Finding, 0, 5)
	if len(warnings) > 0 {
		findings = append(findings, Finding{
			ID:            "quality-collector-error",
			Severity:      "p0",
			Title:         "Quality evidence collection failed",
			Blocking:      true,
			Evidence:      append([]string(nil), warnings...),
			Remediation:   "Repair the failing collector before relying on repository health.",
			VerifyCommand: "./bin/agent-harness quality inspect --json",
		})
	}
	if len(pioneer.BenchmarkMissing) > 0 || len(pioneer.ReproductionMissing) > 0 {
		evidence := make([]string, 0, len(pioneer.BenchmarkMissing)+len(pioneer.ReproductionMissing))
		for _, name := range pioneer.BenchmarkMissing {
			evidence = append(evidence, "benchmark missing: "+name)
		}
		for _, name := range pioneer.ReproductionMissing {
			evidence = append(evidence, "reproduction missing: "+name)
		}
		findings = append(findings, Finding{
			ID:            "pioneer-skill-coverage",
			Severity:      "p1",
			Title:         "Canonical pioneer skill evaluation is incomplete",
			Evidence:      evidence,
			Remediation:   "Add one benchmark fixture and one honest reproduction case for every canonical pioneer skill.",
			VerifyCommand: "./bin/agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json",
		})
	}
	if pioneer.IsolatedExpected > 0 &&
		(pioneer.IsolatedObserved != pioneer.IsolatedExpected || pioneer.IsolatedFailed > 0) {
		findings = append(findings, Finding{
			ID:            "pioneer-isolated-evaluation-incomplete",
			Severity:      "p0",
			Title:         "Fresh-context pioneer evaluation evidence is incomplete or failed",
			Blocking:      true,
			Evidence:      pioneerIsolatedEvidence(pioneer),
			Remediation:   "Run every canonical pioneer fixture in an isolated context and record hashes and verdicts in the evaluation manifest.",
			VerifyCommand: "./bin/agent-harness quality inspect --json",
		})
	} else if pioneer.IsolatedBlocked > 0 {
		findings = append(findings, Finding{
			ID:            "pioneer-isolated-evaluation-blocked",
			Severity:      "p1",
			Title:         "Fresh-context pioneer evaluation has capability-blocked cases",
			Evidence:      pioneerIsolatedEvidence(pioneer),
			Remediation:   "Re-run blocked cases only when the named host capability is available; do not relabel them as pass.",
			VerifyCommand: "./bin/agent-harness quality inspect --json",
		})
	}
	if len(lowCoverage) > 0 {
		findings = append(findings, Finding{
			ID:            "low-coverage-packages",
			Severity:      "p2",
			Title:         "Packages remain below the coverage observation threshold",
			Evidence:      boundedEvidence(coverageEvidence(lowCoverage), 50),
			Remediation:   "Add behavior-focused boundary tests to the highest-risk packages before promotion.",
			VerifyCommand: "go test -cover ./... -count=1",
		})
	}
	highBranchCount := 0
	for _, function := range branchFunctions {
		if function.Branches > 12 {
			highBranchCount++
		}
	}
	if highBranchCount > 0 {
		findings = append(findings, Finding{
			ID:            "high-branch-functions",
			Severity:      "p2",
			Title:         "High-branch production functions need targeted review",
			Evidence:      branchEvidence(branchFunctions, 12),
			Remediation:   "Review the listed functions for missing invariants and add focused regression tests.",
			VerifyCommand: "./bin/agent-harness quality inspect --json",
		})
	}
	if len(auditItems) > 0 {
		severity := "p1"
		blocking := false
		for _, item := range auditItems {
			if item.Priority == "P0" {
				severity = "p0"
				blocking = true
				break
			}
		}
		findings = append(findings, Finding{
			ID:            "project-audit-items",
			Severity:      severity,
			Title:         "P0, P1, or P2 project audit items remain open",
			Blocking:      blocking,
			Evidence:      auditEvidence(auditItems),
			Remediation:   "Resolve or explicitly reclassify the referenced project audit items.",
			VerifyCommand: "./bin/agent-harness quality inspect --json",
		})
	}
	return findings
}

func boundedEvidence(evidence []string, limit int) []string {
	if len(evidence) <= limit {
		return evidence
	}
	return append([]string(nil), evidence[:limit]...)
}

func qualityStatuses(warnings []string, findings []Finding) (string, string, string) {
	if len(warnings) > 0 {
		return CollectionStatusError, HealthStatusUnknown, GateStatusBlock
	}
	if len(findings) == 0 {
		return CollectionStatusOK, HealthStatusHealthy, GateStatusPass
	}
	for _, finding := range findings {
		if finding.Blocking {
			return CollectionStatusOK, HealthStatusNeedsAttention, GateStatusBlock
		}
	}
	return CollectionStatusOK, HealthStatusNeedsAttention, GateStatusReportOnly
}
