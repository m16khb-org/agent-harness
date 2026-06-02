# IssueOps Autoresearch Quality Loop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a measurable IssueOps autoresearch quality gate that evaluates a research candidate against stored baseline/candidate benchmark runs, edit-surface constraints, and keep/discard rules.

**Architecture:** Reuse the existing IssueOps benchmark run/compare model. Add a small core candidate contract plus gate evaluator, then expose it as `agent-harness issueops benchmark gate` so agents can make a deterministic keep/discard decision before accepting an IssueOps improvement candidate.

**Tech Stack:** Go core and CLI in `internal/core` and `cmd/harness`; JSON candidate files; existing IssueOps benchmark state; existing usage/response contract golden tests.

**IssueOps Context:** Issue https://github.com/example/agent-harness/issues/9. Worktree must be `/tmp/agent-harness.worktrees/feature-9-issueops-autoresearch-quality-loop` on branch `feature/9-issueops-autoresearch-quality-loop`.

---

## File Structure

- Modify: `internal/core/issueops_benchmark.go`  
  Add candidate/gate DTOs, edit-surface matching, target-dimension regression checks, and keep/discard evaluation.
- Modify: `internal/core/issueops_benchmark_test.go`  
  Add focused unit tests for passing gate, edit surface violation, target dimension regression, and non-passing candidate benchmark.
- Modify: `cmd/harness/issueops.go`  
  Add `issueops benchmark gate` CLI parsing and JSON/text output.
- Modify: `cmd/harness/issueops_benchmark_test.go`  
  Add CLI tests for gate JSON success and discard reasons.
- Modify: `internal/adapter/cli/usage.go` and `cmd/harness/testdata/usage.golden.txt`  
  Add command usage text.
- Modify: `cmd/harness/contract.go` and `cmd/harness/testdata/response_contracts.golden.json` if the response contract generator requires a new sample.
- Modify: `skills/issueops/SKILL.md`  
  Document the IssueOps autoresearch candidate brief and quality gate command.

---

### Task 1: Core Gate Tests

**Files:**
- Modify: `internal/core/issueops_benchmark_test.go`

- [ ] **Step 1: Verify worker context before editing**

Run:

```bash
pwd
git branch --show-current
git rev-parse --short HEAD
test "$PWD" = "/tmp/agent-harness.worktrees/feature-9-issueops-autoresearch-quality-loop"
test "$(git branch --show-current)" = "feature/9-issueops-autoresearch-quality-loop"
```

Expected: all commands exit `0`.

- [ ] **Step 2: Add failing core tests**

Append these tests to `internal/core/issueops_benchmark_test.go`:

```go
func TestEvaluateIssueOpsAutoresearchGateKeepsPassingCandidate(t *testing.T) {
	candidate := IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "A candidate with bounded files and no benchmark regression should be kept.",
		TargetDimensions: []string{"issue_quality", "plan_quality"},
		EditSurface:      []string{"skills/issueops/**", "internal/core/issueops_benchmark.go"},
		KeepCriteria:     "no regressions and no critical failures",
		DiscardCriteria:  "discard on benchmark regression or edit-surface violation",
	}
	baseline := issueOpsBenchmarkRunForGateTest("baseline", 100, 100, 0)
	next := issueOpsBenchmarkRunForGateTest("candidate", 100, 100, 0)

	result := EvaluateIssueOpsAutoresearchGate(IssueOpsAutoresearchGateRequest{
		Candidate:    candidate,
		BaselineRun:  baseline,
		CandidateRun: next,
		ChangedPaths: []string{"skills/issueops/SKILL.md", "internal/core/issueops_benchmark.go"},
	})

	if !result.OK || !result.KeepCandidate || len(result.DiscardReasons) != 0 {
		t.Fatalf("expected gate to keep candidate: %+v", result)
	}
}

func TestEvaluateIssueOpsAutoresearchGateRejectsEditSurfaceViolation(t *testing.T) {
	candidate := IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "A candidate cannot touch files outside the declared surface.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	}

	result := EvaluateIssueOpsAutoresearchGate(IssueOpsAutoresearchGateRequest{
		Candidate:    candidate,
		BaselineRun:  issueOpsBenchmarkRunForGateTest("baseline", 100, 100, 0),
		CandidateRun: issueOpsBenchmarkRunForGateTest("candidate", 100, 100, 0),
		ChangedPaths: []string{"cmd/harness/issueops.go"},
	})

	if result.KeepCandidate || len(result.EditSurfaceViolations) != 1 || !containsFold(strings.Join(result.DiscardReasons, "\n"), "edit surface") {
		t.Fatalf("expected edit surface discard: %+v", result)
	}
}

func TestEvaluateIssueOpsAutoresearchGateRejectsTargetRegression(t *testing.T) {
	candidate := IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Target dimensions cannot regress.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	}
	baseline := issueOpsBenchmarkRunWithDimensionForGateTest("baseline", "issue_quality", 100)
	next := issueOpsBenchmarkRunWithDimensionForGateTest("candidate", "issue_quality", 90)

	result := EvaluateIssueOpsAutoresearchGate(IssueOpsAutoresearchGateRequest{
		Candidate:    candidate,
		BaselineRun:  baseline,
		CandidateRun: next,
		ChangedPaths: []string{"skills/issueops/SKILL.md"},
	})

	if result.KeepCandidate || len(result.TargetDimensionRegressions) != 1 {
		t.Fatalf("expected target dimension regression discard: %+v", result)
	}
}

func TestEvaluateIssueOpsAutoresearchGateRejectsNonPassingCandidateRun(t *testing.T) {
	candidate := IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Candidate benchmark must pass.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	}

	result := EvaluateIssueOpsAutoresearchGate(IssueOpsAutoresearchGateRequest{
		Candidate:    candidate,
		BaselineRun:  issueOpsBenchmarkRunForGateTest("baseline", 100, 100, 0),
		CandidateRun: issueOpsBenchmarkRunForGateTest("candidate", 90, 90, 1),
		ChangedPaths: []string{"skills/issueops/SKILL.md"},
	})

	if result.KeepCandidate || !containsFold(strings.Join(result.DiscardReasons, "\n"), "candidate benchmark did not pass") {
		t.Fatalf("expected non-passing benchmark discard: %+v", result)
	}
}

func issueOpsBenchmarkRunForGateTest(id string, average, minimum float64, criticalFailures int) IssueOpsBenchmarkRunResult {
	score := IssueOpsBenchmarkScore{
		OK:           criticalFailures == 0 && minimum >= 100,
		FixtureID:    "fixture",
		AverageScore: average,
		MinimumScore: minimum,
		DimensionScores: []IssueOpsDimensionScore{
			{Dimension: "issue_quality", Score: minimum, Evidence: "gate test"},
			{Dimension: "plan_quality", Score: minimum, Evidence: "gate test"},
		},
		Passed: criticalFailures == 0 && minimum >= 100,
	}
	for i := 0; i < criticalFailures; i++ {
		score.CriticalFailures = append(score.CriticalFailures, "critical failure")
	}
	return FinalizeIssueOpsBenchmarkRunResult(IssueOpsBenchmarkRunResult{ID: id, Scores: []IssueOpsBenchmarkScore{score}})
}

func issueOpsBenchmarkRunWithDimensionForGateTest(id, dimension string, scoreValue float64) IssueOpsBenchmarkRunResult {
	score := IssueOpsBenchmarkScore{
		OK:           scoreValue >= 100,
		FixtureID:    "fixture",
		AverageScore: scoreValue,
		MinimumScore: scoreValue,
		DimensionScores: []IssueOpsDimensionScore{
			{Dimension: dimension, Score: scoreValue, Evidence: "gate test"},
		},
		Passed: scoreValue >= 100,
	}
	return FinalizeIssueOpsBenchmarkRunResult(IssueOpsBenchmarkRunResult{ID: id, Scores: []IssueOpsBenchmarkScore{score}})
}
```

- [ ] **Step 3: Run tests to confirm red**

Run:

```bash
go test ./internal/core -run AutoresearchGate -count=1
```

Expected: FAIL with undefined `IssueOpsAutoresearchCandidate`, `IssueOpsAutoresearchGateRequest`, or `EvaluateIssueOpsAutoresearchGate`.

---

### Task 2: Core Gate Implementation

**Files:**
- Modify: `internal/core/issueops_benchmark.go`

- [ ] **Step 1: Add DTOs after `IssueOpsBenchmarkCompareResult`**

Add:

```go
type IssueOpsAutoresearchCandidate struct {
	ID               string   `json:"id"`
	Hypothesis       string   `json:"hypothesis"`
	TargetDimensions []string `json:"target_dimensions"`
	EditSurface      []string `json:"edit_surface"`
	BaselineCommand  string   `json:"baseline_command,omitempty"`
	CandidateCommand string   `json:"candidate_command,omitempty"`
	KeepCriteria     string   `json:"keep_criteria,omitempty"`
	DiscardCriteria  string   `json:"discard_criteria,omitempty"`
}

type IssueOpsAutoresearchGateRequest struct {
	Candidate    IssueOpsAutoresearchCandidate
	BaselineRun  IssueOpsBenchmarkRunResult
	CandidateRun IssueOpsBenchmarkRunResult
	ChangedPaths []string
}

type IssueOpsAutoresearchGateResult struct {
	OK                         bool                           `json:"ok"`
	KeepCandidate              bool                           `json:"keep_candidate"`
	CandidateID                string                         `json:"candidate_id"`
	BenchmarkCompare           IssueOpsBenchmarkCompareResult `json:"benchmark_compare"`
	EditSurfaceViolations      []string                       `json:"edit_surface_violations,omitempty"`
	TargetDimensionRegressions []string                       `json:"target_dimension_regressions,omitempty"`
	DiscardReasons             []string                       `json:"discard_reasons,omitempty"`
}
```

- [ ] **Step 2: Add evaluator near `CompareIssueOpsBenchmarkRuns`**

Add:

```go
func EvaluateIssueOpsAutoresearchGate(req IssueOpsAutoresearchGateRequest) IssueOpsAutoresearchGateResult {
	compare := CompareIssueOpsBenchmarkRuns(req.BaselineRun, req.CandidateRun)
	result := IssueOpsAutoresearchGateResult{
		OK:               true,
		KeepCandidate:    true,
		CandidateID:      strings.TrimSpace(req.Candidate.ID),
		BenchmarkCompare: compare,
	}
	if result.CandidateID == "" {
		result.DiscardReasons = append(result.DiscardReasons, "candidate id is required")
	}
	if strings.TrimSpace(req.Candidate.Hypothesis) == "" {
		result.DiscardReasons = append(result.DiscardReasons, "candidate hypothesis is required")
	}
	if len(req.Candidate.TargetDimensions) == 0 {
		result.DiscardReasons = append(result.DiscardReasons, "target dimensions are required")
	}
	if len(req.Candidate.EditSurface) == 0 {
		result.DiscardReasons = append(result.DiscardReasons, "edit surface is required")
	}
	if !req.CandidateRun.OK {
		result.DiscardReasons = append(result.DiscardReasons, "candidate benchmark did not pass")
	}
	if !compare.OK {
		result.DiscardReasons = append(result.DiscardReasons, "benchmark comparison regressed")
	}
	result.EditSurfaceViolations = issueOpsEditSurfaceViolations(req.ChangedPaths, req.Candidate.EditSurface)
	if len(result.EditSurfaceViolations) > 0 {
		result.DiscardReasons = append(result.DiscardReasons, "changed paths outside declared edit surface")
	}
	result.TargetDimensionRegressions = issueOpsTargetDimensionRegressions(req.Candidate.TargetDimensions, req.BaselineRun, req.CandidateRun)
	if len(result.TargetDimensionRegressions) > 0 {
		result.DiscardReasons = append(result.DiscardReasons, "target dimensions regressed")
	}
	result.KeepCandidate = len(result.DiscardReasons) == 0
	result.OK = result.KeepCandidate
	return result
}
```

- [ ] **Step 3: Add helper functions near dimension comparison helpers**

Add:

```go
func issueOpsTargetDimensionRegressions(targets []string, baseline, candidate IssueOpsBenchmarkRunResult) []string {
	baselineScores := issueOpsDimensionMinimums(baseline)
	candidateScores := issueOpsDimensionMinimums(candidate)
	var regressions []string
	for _, target := range targets {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		if candidateScores[target] < baselineScores[target] {
			regressions = append(regressions, target)
		}
	}
	return regressions
}

func issueOpsEditSurfaceViolations(changedPaths, editSurface []string) []string {
	if len(changedPaths) == 0 || len(editSurface) == 0 {
		return nil
	}
	var violations []string
	for _, changedPath := range changedPaths {
		changedPath = normalizeIssueOpsPath(changedPath)
		if changedPath == "" {
			continue
		}
		if !issueOpsPathAllowed(changedPath, editSurface) {
			violations = append(violations, changedPath)
		}
	}
	return violations
}

func issueOpsPathAllowed(changedPath string, editSurface []string) bool {
	for _, pattern := range editSurface {
		pattern = normalizeIssueOpsPath(pattern)
		if pattern == "" {
			continue
		}
		if strings.HasSuffix(pattern, "/**") {
			prefix := strings.TrimSuffix(pattern, "/**")
			if changedPath == prefix || strings.HasPrefix(changedPath, prefix+"/") {
				return true
			}
			continue
		}
		if ok, _ := filepath.Match(pattern, changedPath); ok {
			return true
		}
		if changedPath == pattern {
			return true
		}
	}
	return false
}

func normalizeIssueOpsPath(path string) string {
	path = strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	path = strings.TrimPrefix(path, "./")
	return strings.Trim(path, "/")
}
```

Also ensure `path/filepath` is already imported. It is currently used in the file, so no import change should be needed.

- [ ] **Step 4: Run focused tests**

Run:

```bash
go test ./internal/core -run 'AutoresearchGate|IssueOpsBenchmark' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit core gate**

Run:

```bash
git add internal/core/issueops_benchmark.go internal/core/issueops_benchmark_test.go
git commit -m "feat(issueops): add autoresearch quality gate" -m "Lore:
- Intent: Add a deterministic keep/discard gate for IssueOps autoresearch candidates.
- Why: Issue #9 needs candidate quality decisions to depend on benchmark comparison and edit-surface constraints.
- Changes:
  - Add candidate and gate DTOs.
  - Evaluate benchmark regressions, target dimensions, candidate pass status, and edit surface violations.
  - Cover pass and discard paths with focused tests.
- Verify: go test ./internal/core -run 'AutoresearchGate|IssueOpsBenchmark' -count=1
- Risk: Low; core helper is additive."
```

---

### Task 3: CLI Gate Command

**Files:**
- Modify: `cmd/harness/issueops.go`
- Modify: `cmd/harness/issueops_benchmark_test.go`

- [ ] **Step 1: Add failing CLI tests**

Append to `cmd/harness/issueops_benchmark_test.go`:

```go
func TestRunIssueOpsBenchmarkGateCLIKeepsCandidate(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	baseline := core.IssueOpsBenchmarkRunResult{
		ID: "baseline",
		Scores: []core.IssueOpsBenchmarkScore{{
			OK:           true,
			FixtureID:    "fixture",
			AverageScore: 100,
			MinimumScore: 100,
			DimensionScores: []core.IssueOpsDimensionScore{
				{Dimension: "issue_quality", Score: 100, Evidence: "baseline"},
			},
			Passed: true,
		}},
	}
	candidateRun := baseline
	candidateRun.ID = "candidate"
	if err := core.SaveIssueOpsBenchmarkRun(stateDir, core.FinalizeIssueOpsBenchmarkRunResult(baseline)); err != nil {
		t.Fatal(err)
	}
	if err := core.SaveIssueOpsBenchmarkRun(stateDir, core.FinalizeIssueOpsBenchmarkRunResult(candidateRun)); err != nil {
		t.Fatal(err)
	}
	candidatePath := writeIssueOpsCandidateForCLITest(t, core.IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Bounded IssueOps changes should pass the gate.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	})

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "gate", "--baseline", "baseline", "--candidate", "candidate", "--candidate-file", candidatePath, "--changed-path", "skills/issueops/SKILL.md", "--json"})
	})
	var result core.IssueOpsAutoresearchGateResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("gate should return JSON: %v\n%s", err, out)
	}
	if !result.OK || !result.KeepCandidate {
		t.Fatalf("expected gate to keep candidate: %+v", result)
	}
}

func TestRunIssueOpsBenchmarkGateCLIDiscardsOutsideEditSurface(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	run := core.FinalizeIssueOpsBenchmarkRunResult(core.IssueOpsBenchmarkRunResult{
		ID: "baseline",
		Scores: []core.IssueOpsBenchmarkScore{{
			OK:           true,
			FixtureID:    "fixture",
			AverageScore: 100,
			MinimumScore: 100,
			DimensionScores: []core.IssueOpsDimensionScore{
				{Dimension: "issue_quality", Score: 100, Evidence: "baseline"},
			},
			Passed: true,
		}},
	})
	if err := core.SaveIssueOpsBenchmarkRun(stateDir, run); err != nil {
		t.Fatal(err)
	}
	candidatePath := writeIssueOpsCandidateForCLITest(t, core.IssueOpsAutoresearchCandidate{
		ID:               "issueops-autoresearch-loop",
		Hypothesis:       "Only skill changes are allowed.",
		TargetDimensions: []string{"issue_quality"},
		EditSurface:      []string{"skills/issueops/**"},
	})

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "gate", "--baseline", "baseline", "--candidate", "baseline", "--candidate-file", candidatePath, "--changed-path", "cmd/harness/issueops.go", "--json"})
	})
	var result core.IssueOpsAutoresearchGateResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("gate should return JSON: %v\n%s", err, out)
	}
	if result.KeepCandidate || len(result.EditSurfaceViolations) != 1 {
		t.Fatalf("expected edit-surface discard: %+v", result)
	}
}

func writeIssueOpsCandidateForCLITest(t *testing.T, candidate core.IssueOpsAutoresearchCandidate) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "candidate.json")
	b, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
```

Add `agent-harness/internal/core` to the test imports if it is not already imported:

```go
import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/core"
)
```

- [ ] **Step 2: Confirm CLI tests are red**

Run:

```bash
go test ./cmd/harness -run IssueOpsBenchmarkGate -count=1
```

Expected: FAIL with unknown `benchmark` subcommand `gate` or missing core DTO imports.

- [ ] **Step 3: Add candidate loader and gate command**

In `cmd/harness/issueops.go`, add imports:

```go
import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"agent-harness/internal/core"
)
```

Add a new `case "gate"` inside `runIssueOpsBenchmark`:

```go
	case "gate":
		fs := flag.NewFlagSet("issueops benchmark gate", flag.ContinueOnError)
		baselineID := fs.String("baseline", "", "baseline benchmark id")
		candidateID := fs.String("candidate", "", "candidate benchmark id")
		candidateFile := fs.String("candidate-file", "", "IssueOps autoresearch candidate JSON file")
		var changedPaths repeatedFlag
		fs.Var(&changedPaths, "changed-path", "changed path to check against the candidate edit surface; repeatable")
		jsonOut := fs.Bool("json", false, "print JSON")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		candidate, err := readIssueOpsAutoresearchCandidateFile(*candidateFile)
		if err != nil {
			return err
		}
		baseline, err := core.ReadIssueOpsBenchmarkRun(core.StateDir(), *baselineID)
		if err != nil {
			return err
		}
		candidateRun, err := core.ReadIssueOpsBenchmarkRun(core.StateDir(), *candidateID)
		if err != nil {
			return err
		}
		result := core.EvaluateIssueOpsAutoresearchGate(core.IssueOpsAutoresearchGateRequest{
			Candidate:    candidate,
			BaselineRun:  baseline,
			CandidateRun: candidateRun,
			ChangedPaths: changedPaths,
		})
		if *jsonOut {
			return printJSON(result)
		}
		fmt.Printf("keep_candidate=%v ok=%v discard_reasons=%d\n", result.KeepCandidate, result.OK, len(result.DiscardReasons))
		for _, reason := range result.DiscardReasons {
			fmt.Printf("- discard: %s\n", reason)
		}
		return nil
```

Add helper types/functions near the bottom of `cmd/harness/issueops.go`:

```go
type repeatedFlag []string

func (f *repeatedFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *repeatedFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func readIssueOpsAutoresearchCandidateFile(path string) (core.IssueOpsAutoresearchCandidate, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return core.IssueOpsAutoresearchCandidate{}, fmt.Errorf("candidate-file is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return core.IssueOpsAutoresearchCandidate{}, err
	}
	var candidate core.IssueOpsAutoresearchCandidate
	if err := json.Unmarshal(b, &candidate); err != nil {
		return core.IssueOpsAutoresearchCandidate{}, fmt.Errorf("parse candidate file %s: %w", path, err)
	}
	return candidate, nil
}
```

- [ ] **Step 4: Run CLI focused tests**

Run:

```bash
go test ./cmd/harness -run IssueOpsBenchmarkGate -count=1
```

Expected: PASS.

---

### Task 4: Usage, Contracts, And Skill Docs

**Files:**
- Modify: `internal/adapter/cli/usage.go`
- Modify: `cmd/harness/testdata/usage.golden.txt`
- Modify: `cmd/harness/contract.go`
- Modify: `cmd/harness/testdata/response_contracts.golden.json`
- Modify: `skills/issueops/SKILL.md`

- [ ] **Step 1: Add usage line**

In `internal/adapter/cli/usage.go`, add this line after benchmark compare usage:

```text
  agent-harness issueops benchmark gate --baseline KEY --candidate KEY --candidate-file PATH [--changed-path PATH]... [--json]
```

Run:

```bash
go test ./cmd/harness -run TestUsageGolden -count=1
```

Expected: FAIL until `cmd/harness/testdata/usage.golden.txt` is updated.

- [ ] **Step 2: Update usage golden**

Add the same usage line to `cmd/harness/testdata/usage.golden.txt`.

Run:

```bash
go test ./cmd/harness -run TestUsageGolden -count=1
```

Expected: PASS.

- [ ] **Step 3: Check response contracts**

Run:

```bash
go test ./cmd/harness -run TestResponseContractsGolden -count=1
```

Expected: PASS if no response contract sample is needed. If it fails because `issueops benchmark gate` is part of the command contract, update `cmd/harness/contract.go` with a compact fake-state sample and refresh `cmd/harness/testdata/response_contracts.golden.json` using the repo's existing golden update path.

- [ ] **Step 4: Document the gate in the IssueOps skill**

In `skills/issueops/SKILL.md`, add this section after "Run the 100-point quality benchmark":

```markdown
Run the autoresearch keep/discard gate for IssueOps improvement candidates:

```bash
agent-harness issueops benchmark gate --baseline "$BASELINE_ID" --candidate "$CANDIDATE_ID" --candidate-file candidate.json --changed-path skills/issueops/SKILL.md --json
```

The candidate file records the hypothesis, target dimensions, edit surface, and keep/discard criteria. The gate keeps a candidate only when the candidate benchmark passes, baseline comparison has no regression, target dimensions do not regress, and every changed path is inside the declared edit surface.
```

- [ ] **Step 5: Run docs/skill validation**

Run:

```bash
python3 ${CODEX_HOME:-$HOME/.codex}/skills/.system/skill-creator/scripts/quick_validate.py skills/issueops
```

Expected: PASS.

- [ ] **Step 6: Commit CLI and docs**

Run:

```bash
git add cmd/harness/issueops.go cmd/harness/issueops_benchmark_test.go internal/adapter/cli/usage.go cmd/harness/testdata/usage.golden.txt cmd/harness/contract.go cmd/harness/testdata/response_contracts.golden.json skills/issueops/SKILL.md
git commit -m "feat(issueops): expose autoresearch benchmark gate" -m "Lore:
- Intent: Make the IssueOps autoresearch keep/discard gate available from the CLI and skill workflow.
- Why: Issue #9 needs agents to evaluate candidates from stored benchmark runs and declared edit surfaces.
- Changes:
  - Add issueops benchmark gate CLI.
  - Document candidate files and gate criteria in the IssueOps skill.
  - Update usage and response contract artifacts when required.
- Verify: go test ./cmd/harness -run 'IssueOpsBenchmarkGate|TestUsageGolden|TestResponseContractsGolden' -count=1; quick_validate.py skills/issueops
- Risk: Low; command is additive and deterministic."
```

---

### Task 5: End-To-End Verification And IssueOps Links

**Files:**
- Modify: IssueOps state only through CLI.

- [ ] **Step 1: Link this plan**

Run from the worktree:

```bash
agent-harness issueops link-plan --id io-b1ed844aea3e --plan-path docs/superpowers/plans/2026-06-02-issueops-autoresearch-quality-loop.md --json
```

Expected: JSON with `phase` at or beyond `implementation` and `plan_path` set.

- [ ] **Step 2: Run full verification**

Run:

```bash
go test ./internal/core -run IssueOps -count=1
go test ./cmd/harness -run IssueOps -count=1
go test ./cmd/harness -run Golden -count=1
go test ./... -count=1
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json
```

Expected: all commands exit `0`; benchmark JSON has `ok: true`, `average_score: 100`, `minimum_score: 100`, and `critical_failure_count: 0`.

- [ ] **Step 3: Run an actual gate smoke**

Create a temporary candidate file:

```bash
tmp_candidate="$(mktemp)"
cat > "$tmp_candidate" <<'JSON'
{
  "id": "issueops-autoresearch-loop",
  "hypothesis": "The IssueOps skill and gate command improve candidate quality control.",
  "target_dimensions": ["issue_quality", "plan_quality"],
  "edit_surface": ["skills/issueops/**", "internal/core/**", "cmd/harness/**", "internal/adapter/cli/**", "cmd/harness/testdata/**", "docs/superpowers/**"],
  "baseline_command": "agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json",
  "candidate_command": "agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json",
  "keep_criteria": "candidate passes and no benchmark or target dimension regression",
  "discard_criteria": "discard on critical failure, regression, or edit-surface violation"
}
JSON
```

Run two benchmark runs and gate them:

```bash
baseline_json="$(./bin/agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json)"
candidate_json="$(./bin/agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json)"
baseline_id="$(printf '%s' "$baseline_json" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
candidate_id="$(printf '%s' "$candidate_json" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')"
./bin/agent-harness issueops benchmark gate --baseline "$baseline_id" --candidate "$candidate_id" --candidate-file "$tmp_candidate" --changed-path skills/issueops/SKILL.md --changed-path internal/core/issueops_benchmark.go --json
rm -f "$tmp_candidate"
```

Expected: gate JSON has `ok: true` and `keep_candidate: true`.

- [ ] **Step 4: Check PR readiness**

Run:

```bash
agent-harness issueops pr-readiness --id io-b1ed844aea3e --json
```

Expected: no missing `issue_url` or `plan_path`. If PR readiness reports other missing fields, record them and complete the required step before PR drafting.

- [ ] **Step 5: Final review**

Run:

```bash
git status --short
git log --oneline --decorate -3
```

Expected: only intended tracked changes are present, and commits are on `feature/9-issueops-autoresearch-quality-loop`.
