# IssueOps Quality Benchmark Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an IssueOps quality benchmark that scores issue-driven workflow artifacts with deterministic checks and an `agy -p` JSON judge, then compares baseline and candidate runs.

**Architecture:** Keep benchmark logic in focused core files under `internal/core`, expose it through `agent-harness issueops benchmark ...`, and reuse existing response contract/golden patterns. Fixtures are source-controlled JSON; benchmark results are compact harness state records.

**Tech Stack:** Go core/CLI in `internal/core` and `cmd/harness`; JSON fixtures under `testdata/issueops/fixtures`; `agy -p` via bounded `exec.CommandContext`; existing golden tests and state helpers.

---

### Task 1: Fixture Schema And Sample Fixtures

**Files:**
- Create: `internal/core/issueops_benchmark.go`
- Create: `internal/core/issueops_benchmark_test.go`
- Create: `testdata/issueops/fixtures/ambiguous-intent.json`
- Create: `testdata/issueops/fixtures/worktree-gate.json`

- [ ] **Step 1: Write failing fixture loader tests**

Create `internal/core/issueops_benchmark_test.go` with:

```go
package core

import (
	"path/filepath"
	"testing"
)

func TestLoadIssueOpsBenchmarkFixtures(t *testing.T) {
	fixtures, err := LoadIssueOpsBenchmarkFixtures(filepath.Join("..", "..", "testdata", "issueops", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) < 2 {
		t.Fatalf("expected at least two fixtures, got %d", len(fixtures))
	}
	for _, fixture := range fixtures {
		if fixture.ID == "" || fixture.Title == "" || fixture.UserPrompt == "" {
			t.Fatalf("fixture missing required fields: %+v", fixture)
		}
		if len(fixture.CriticalFailures) == 0 {
			t.Fatalf("fixture %s should define critical failures", fixture.ID)
		}
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```bash
go test ./internal/core -run TestLoadIssueOpsBenchmarkFixtures -count=1
```

Expected: FAIL with `undefined: LoadIssueOpsBenchmarkFixtures`.

- [ ] **Step 3: Implement fixture types and loader**

Create `internal/core/issueops_benchmark.go` with:

```go
package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type IssueOpsBenchmarkFixture struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	UserPrompt       string   `json:"user_prompt"`
	RepoContext      string   `json:"repo_context"`
	ExpectedIssue    []string `json:"expected_issue"`
	ExpectedPlan     []string `json:"expected_plan"`
	ExpectedTasks    []string `json:"expected_tasks"`
	ExpectedTDD      []string `json:"expected_tdd"`
	ExpectedSubagents []string `json:"expected_subagents"`
	ExpectedPR       []string `json:"expected_pr"`
	CriticalFailures []string `json:"critical_failures"`
}

func LoadIssueOpsBenchmarkFixtures(dir string) ([]IssueOpsBenchmarkFixture, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("fixtures path is required")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var fixtures []IssueOpsBenchmarkFixture
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var fixture IssueOpsBenchmarkFixture
		if err := json.Unmarshal(b, &fixture); err != nil {
			return nil, fmt.Errorf("parse fixture %s: %w", path, err)
		}
		if err := validateIssueOpsBenchmarkFixture(fixture); err != nil {
			return nil, fmt.Errorf("invalid fixture %s: %w", path, err)
		}
		fixtures = append(fixtures, fixture)
	}
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].ID < fixtures[j].ID })
	if len(fixtures) == 0 {
		return nil, fmt.Errorf("no issueops benchmark fixtures in %s", dir)
	}
	return fixtures, nil
}

func validateIssueOpsBenchmarkFixture(f IssueOpsBenchmarkFixture) error {
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(f.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(f.UserPrompt) == "" {
		return fmt.Errorf("user_prompt is required")
	}
	if strings.TrimSpace(f.RepoContext) == "" {
		return fmt.Errorf("repo_context is required")
	}
	if len(f.CriticalFailures) == 0 {
		return fmt.Errorf("critical_failures is required")
	}
	return nil
}
```

- [ ] **Step 4: Add sample fixtures**

Create `testdata/issueops/fixtures/ambiguous-intent.json`:

```json
{
  "id": "ambiguous-intent",
  "title": "Ambiguous user intent requires clarification",
  "user_prompt": "IssueOps를 더 좋게 만들어줘",
  "repo_context": "agent-harness has an issueops skill, CLI state helpers, MCP tools, and project docs. The request is too broad to implement safely without clarifying what quality means.",
  "expected_issue": ["states ambiguity", "defines acceptance criteria", "lists non-goals", "includes verification", "is written in Korean", "references issue/pr guideline"],
  "expected_plan": ["starts with measurement", "does not optimize prompts first", "uses testable tasks"],
  "expected_tasks": ["separates fixture schema", "separates deterministic scoring", "separates judge adapter"],
  "expected_tdd": ["writes failing tests before benchmark implementation"],
  "expected_subagents": ["assigns non-overlapping file ownership"],
  "expected_pr": ["summarizes benchmark evidence", "links issue", "lists verification", "is written in Korean", "references issue/pr guideline"],
  "critical_failures": ["implements before clarifying quality metric", "skips issue contract", "skips verification", "issue or pr/mr not written in Korean", "missing issue/pr guideline reference", "excessive emoji in issue or pr/mr"]
}
```

Create `testdata/issueops/fixtures/worktree-gate.json`:

```json
{
  "id": "worktree-gate",
  "title": "Issue branch requires isolated worktree",
  "user_prompt": "이슈 #1 기반으로 구현해줘",
  "repo_context": "The user requires issue-based branches and sibling worktrees under <repo>.worktrees/<branch-slug>. Implementation must not happen in the source repo.",
  "expected_issue": ["links issue", "records issue branch requirement", "is written in Korean", "references issue/pr guideline"],
  "expected_plan": ["blocks implementation until branch is provided", "records worktree path"],
  "expected_tasks": ["runs work only in isolated worktree"],
  "expected_tdd": ["runs tests from worktree"],
  "expected_subagents": ["mentions worktree path in prompts", "defines file ownership"],
  "expected_pr": ["uses worktree branch", "includes cleanup status", "is written in Korean", "references issue/pr guideline"],
  "critical_failures": ["works in source repo", "skips branch prompt", "removes dirty worktree", "issue or pr/mr not written in Korean", "missing issue/pr guideline reference", "excessive emoji in issue or pr/mr"]
}
```

- [ ] **Step 5: Verify fixtures**

Run:

```bash
go test ./internal/core -run TestLoadIssueOpsBenchmarkFixtures -count=1
```

Expected: PASS.

### Task 2: Deterministic Scoring

**Files:**
- Modify: `internal/core/issueops_benchmark.go`
- Modify: `internal/core/issueops_benchmark_test.go`

- [ ] **Step 1: Write failing deterministic scorer test**

Add:

```go
func TestScoreIssueOpsBenchmarkArtifactDeterministic(t *testing.T) {
	fixture := IssueOpsBenchmarkFixture{ID: "worktree", CriticalFailures: []string{"works in source repo"}}
	artifact := IssueOpsBenchmarkArtifact{
		IssueDraft: "## Problem\n\n문제 요약\n\n## Current Evidence\n\n현재 근거\n\n## Acceptance Criteria\n\n완료 기준\n\n## Non-goals\n\n비목표\n\n## Verification\n\n검증\n\n## Feedback Log\n\n피드백 기록\n\nGuideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n",
		Plan: "Run: go test ./... -count=1\n",
		TDDPlan: "Write failing test before implementation.\n",
		TaskBreakdown: "Worker A owns internal/core/issueops_benchmark.go. Worker B owns cmd/harness/issueops.go.\n",
		SubagentPrompts: "You are not alone in the codebase. Do not revert others. Own internal/core only.\n",
		PRDraft: "Intent\n의도\nChanges\n변경사항\nVerification\n검증\nRisk\n위험\nReviewer Notes\n리뷰어 참고\nIssue: https://github.com/m16khb/agent-harness/issues/1\nGuideline: docs/superpowers/specs/issueops-issue-pr-guidelines.md\n",
		PhaseChoices: "Proceed to plan | revise current phase | jump to issue | pause",
		BranchName: "feature/1-issueops-quality-benchmark",
		WorktreePath: "/repo.worktrees/feature-1-issueops-quality-benchmark",
		ImplementationLocation: "/repo.worktrees/feature-1-issueops-quality-benchmark",
		WorktreeCleanup: "clean worktree; cleanup offered after merge",
	}
	score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
	if !score.Passed || score.AverageScore < 5 {
		t.Fatalf("expected complete artifact to pass: %+v", score)
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```bash
go test ./internal/core -run TestScoreIssueOpsBenchmarkArtifactDeterministic -count=1
```

Expected: FAIL with undefined scorer types.

- [ ] **Step 3: Implement deterministic score types**

Add:

```go
type IssueOpsBenchmarkArtifact struct {
	ProblemSummary string `json:"problem_summary,omitempty"`
	IssueDraft string `json:"issue_draft,omitempty"`
	Plan string `json:"plan,omitempty"`
	TaskBreakdown string `json:"task_breakdown,omitempty"`
	TDDPlan string `json:"tdd_plan,omitempty"`
	SubagentPrompts string `json:"subagent_prompts,omitempty"`
	ImplementationNotes string `json:"implementation_notes,omitempty"`
	PRDraft string `json:"pr_draft,omitempty"`
	PhaseChoices string `json:"phase_choices,omitempty"`
	BranchName string `json:"branch_name,omitempty"`
	WorktreePath string `json:"worktree_path,omitempty"`
	ImplementationLocation string `json:"implementation_location,omitempty"`
	WorktreeCleanup string `json:"worktree_cleanup,omitempty"`
}

type IssueOpsDimensionScore struct {
	Dimension string `json:"dimension"`
	Score float64 `json:"score"`
	Evidence string `json:"evidence"`
}

type IssueOpsBenchmarkScore struct {
	OK bool `json:"ok"`
	FixtureID string `json:"fixture_id"`
	AverageScore float64 `json:"average_score"`
	MinimumScore float64 `json:"minimum_score"`
	DimensionScores []IssueOpsDimensionScore `json:"dimension_scores"`
	DeterministicFailures []string `json:"deterministic_failures"`
	JudgeFailures []string `json:"judge_failures"`
	CriticalFailures []string `json:"critical_failures"`
	Passed bool `json:"passed"`
}
```

Implement `ScoreIssueOpsBenchmarkArtifact(fixture, artifact)` with these exact dimension names:

```go
var issueOpsBenchmarkDimensions = []string{
	"intent_understanding",
	"issue_quality",
	"plan_quality",
	"task_decomposition",
	"tdd_quality",
	"subagent_orchestration",
	"implementation_readiness",
	"pr_mr_quality",
	"phase_control_quality",
	"branch_worktree_gate_quality",
	"isolation_compliance",
	"worktree_cleanup_quality",
}
```

Use simple text checks for the first implementation. A missing required section, Korean issue/PR text, guideline reference, or gate returns score `0` for that dimension and adds a deterministic failure.

- [ ] **Step 4: Verify deterministic scorer**

Run:

```bash
go test ./internal/core -run 'IssueOpsBenchmark|ScoreIssueOps' -count=1
```

Expected: PASS.

### Task 3: Agy Judge Adapter With Strict JSON

**Files:**
- Create: `internal/core/issueops_benchmark_judge.go`
- Create: `internal/core/issueops_benchmark_judge_test.go`

- [ ] **Step 1: Write fake judge tests**

Create tests:

```go
func TestIssueOpsAgyJudgeParsesStrictJSON(t *testing.T) {
	fake := writeFakeAgy(t, `{"ok":true,"dimension_scores":[{"dimension":"intent_understanding","score":5,"evidence":"matches request"}],"critical_failures":[]}`)
	result, err := RunIssueOpsAgyJudge(IssueOpsAgyJudgeRequest{
		RepoRoot: t.TempDir(),
		AgyCommand: fake,
		Fixture: IssueOpsBenchmarkFixture{ID: "fixture"},
		Artifact: IssueOpsBenchmarkArtifact{ProblemSummary: "summary"},
	})
	if err != nil || !result.OK || len(result.DimensionScores) != 1 {
		t.Fatalf("unexpected judge result err=%v result=%+v", err, result)
	}
}

func TestIssueOpsAgyJudgeRejectsNoisyOutput(t *testing.T) {
	fake := writeFakeAgy(t, `I will judge now. {"ok":true}`)
	_, err := RunIssueOpsAgyJudge(IssueOpsAgyJudgeRequest{RepoRoot: t.TempDir(), AgyCommand: fake, Fixture: IssueOpsBenchmarkFixture{ID: "fixture"}})
	if err == nil {
		t.Fatal("expected strict JSON error")
	}
}
```

- [ ] **Step 2: Run the failing tests**

Run:

```bash
go test ./internal/core -run IssueOpsAgyJudge -count=1
```

Expected: FAIL with undefined judge functions.

- [ ] **Step 3: Implement judge adapter**

Create:

```go
type IssueOpsAgyJudgeRequest struct {
	RepoRoot string
	AgyCommand string
	Timeout time.Duration
	Fixture IssueOpsBenchmarkFixture
	Artifact IssueOpsBenchmarkArtifact
}

func RunIssueOpsAgyJudge(req IssueOpsAgyJudgeRequest) (IssueOpsBenchmarkScore, error)
```

Implementation requirements:

- default command: `agy`,
- default timeout: `2 * time.Minute`,
- execute `agy -p <prompt>` in `RepoRoot`,
- use `json.Decoder` with `DisallowUnknownFields`,
- reject output unless the entire trimmed output is one JSON object,
- return bounded error strings for failures.

- [ ] **Step 4: Verify judge adapter**

Run:

```bash
go test ./internal/core -run IssueOpsAgyJudge -count=1
```

Expected: PASS.

### Task 4: Benchmark Run And Compare Core

**Files:**
- Modify: `internal/core/issueops_benchmark.go`
- Modify: `internal/core/issueops_benchmark_test.go`

- [ ] **Step 1: Write failing run/compare tests**

Add:

```go
func TestRunAndCompareIssueOpsBenchmark(t *testing.T) {
	dir := t.TempDir()
	fixtures := []IssueOpsBenchmarkFixture{{ID: "fixture", Title: "Fixture", UserPrompt: "prompt", RepoContext: "ctx", CriticalFailures: []string{"missing issue"}}}
	baseline := RunIssueOpsBenchmark(IssueOpsBenchmarkRunRequest{StateRoot: dir, Fixtures: fixtures, Artifacts: map[string]IssueOpsBenchmarkArtifact{"fixture": {}}})
	candidate := RunIssueOpsBenchmark(IssueOpsBenchmarkRunRequest{StateRoot: dir, Fixtures: fixtures, Artifacts: map[string]IssueOpsBenchmarkArtifact{"fixture": completeBenchmarkArtifactForTest()}})
	compare := CompareIssueOpsBenchmarkRuns(baseline, candidate)
	if !compare.Improved || compare.AverageScoreDelta <= 0 {
		t.Fatalf("expected candidate improvement: %+v", compare)
	}
}
```

- [ ] **Step 2: Run the failing test**

Run:

```bash
go test ./internal/core -run RunAndCompareIssueOpsBenchmark -count=1
```

Expected: FAIL with undefined run/compare functions.

- [ ] **Step 3: Implement run/compare DTOs**

Add:

```go
type IssueOpsBenchmarkRunRequest struct {
	StateRoot string
	Fixtures []IssueOpsBenchmarkFixture
	Artifacts map[string]IssueOpsBenchmarkArtifact
}

type IssueOpsBenchmarkRunResult struct {
	OK bool `json:"ok"`
	ID string `json:"id"`
	FixtureCount int `json:"fixture_count"`
	AverageScore float64 `json:"average_score"`
	MinimumScore float64 `json:"minimum_score"`
	CriticalFailureCount int `json:"critical_failure_count"`
	Scores []IssueOpsBenchmarkScore `json:"scores"`
}

type IssueOpsBenchmarkCompareResult struct {
	OK bool `json:"ok"`
	Improved bool `json:"improved"`
	BaselineID string `json:"baseline_id"`
	CandidateID string `json:"candidate_id"`
	AverageScoreDelta float64 `json:"average_score_delta"`
	MinimumScoreDelta float64 `json:"minimum_score_delta"`
	CriticalFailureDelta int `json:"critical_failure_delta"`
	Regressions []string `json:"regressions"`
}
```

Persist run results under `<stateRoot>/issueops-benchmarks/<id>.json`.

- [ ] **Step 4: Verify run/compare core**

Run:

```bash
go test ./internal/core -run 'IssueOpsBenchmark|RunAndCompare' -count=1
```

Expected: PASS.

### Task 5: CLI Surface

**Files:**
- Modify: `cmd/harness/issueops.go`
- Create: `cmd/harness/issueops_benchmark_test.go`
- Modify: `internal/adapter/cli/usage.go`
- Modify: `cmd/harness/contract.go`
- Modify: `cmd/harness/response_contract_golden_test.go`
- Modify: `cmd/harness/testdata/usage.golden.txt`
- Modify: `cmd/harness/testdata/response_contracts.golden.json`

- [ ] **Step 1: Write failing CLI tests**

Create `cmd/harness/issueops_benchmark_test.go`:

```go
package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestRunIssueOpsBenchmarkCLI(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"benchmark", "run", "--fixtures", filepath.Join("..", "..", "testdata", "issueops", "fixtures"), "--judge", "none", "--json"})
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("benchmark run should return JSON: %v\n%s", err, out)
	}
	if result["ok"] != true || result["fixture_count"].(float64) < 1 {
		t.Fatalf("unexpected benchmark result: %#v", result)
	}
}
```

- [ ] **Step 2: Run failing CLI test**

Run:

```bash
go test ./cmd/harness -run TestRunIssueOpsBenchmarkCLI -count=1
```

Expected: FAIL with unknown `benchmark` subcommand.

- [ ] **Step 3: Implement CLI parsing**

Add `case "benchmark": return runIssueOpsBenchmark(args[1:])` in `runIssueOps`.

Support:

```bash
agent-harness issueops benchmark run --fixtures PATH --judge none|agy --agy-command agy --json
agent-harness issueops benchmark compare --baseline KEY --candidate KEY --json
```

Use `--judge none` for deterministic-only tests.

- [ ] **Step 4: Update usage and contract fields**

Add usage lines:

```text
agent-harness issueops benchmark run --fixtures PATH [--judge none|agy] [--agy-command PATH] [--json]
agent-harness issueops benchmark compare --baseline KEY --candidate KEY [--json]
```

Add response contract field keys:

```go
"issueops_benchmark_run": {"ok", "id", "fixture_count", "average_score", "minimum_score", "critical_failure_count", "scores"},
"issueops_benchmark_compare": {"ok", "improved", "baseline_id", "candidate_id", "average_score_delta", "minimum_score_delta", "critical_failure_delta", "regressions"},
```

- [ ] **Step 5: Update golden snapshots**

Run:

```bash
go test ./cmd/harness -run Golden -update -count=1
```

Expected: PASS and intentional golden updates.

- [ ] **Step 6: Verify CLI**

Run:

```bash
go test ./cmd/harness -run 'IssueOpsBenchmark|Golden|Contract' -count=1
```

Expected: PASS.

### Task 6: Full Verification And Commit

**Files:**
- All files changed by Tasks 1-5.

- [ ] **Step 1: Run full tests**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 2: Run build**

Run:

```bash
go build -o bin/agent-harness ./cmd/harness
```

Expected: PASS.

- [ ] **Step 3: Run smoke benchmark**

Run:

```bash
tmp_state="$(mktemp -d)" && HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness issueops benchmark run --fixtures testdata/issueops/fixtures --judge none --json && rm -rf "$tmp_state"
```

Expected: JSON contains `"ok": true`, `"fixture_count": 2`, and benchmark scores.

- [ ] **Step 4: Check diff hygiene**

Run:

```bash
git diff --check
git status --short
```

Expected: no whitespace errors; only intended files changed.

- [ ] **Step 5: Commit implementation**

Run:

```bash
git add -- internal/core/issueops_benchmark.go internal/core/issueops_benchmark_test.go internal/core/issueops_benchmark_judge.go internal/core/issueops_benchmark_judge_test.go cmd/harness/issueops.go cmd/harness/issueops_benchmark_test.go internal/adapter/cli/usage.go cmd/harness/contract.go cmd/harness/response_contract_golden_test.go cmd/harness/testdata/usage.golden.txt cmd/harness/testdata/response_contracts.golden.json testdata/issueops/fixtures/ambiguous-intent.json testdata/issueops/fixtures/worktree-gate.json docs/superpowers/plans/2026-06-02-issueops-quality-benchmark.md
git commit -m "feat(issueops): add quality benchmark scoring" -m "Lore:
- Intent: Add deterministic and judge-backed IssueOps quality benchmark scoring.
- Why: IssueOps improvements need measurable quality evidence before prompt or workflow optimization.
- Changes:
  - Add benchmark fixtures, scoring, agy judge adapter, CLI run/compare, and response contracts.
- Verify: go test ./... -count=1; go build -o bin/agent-harness ./cmd/harness; issueops benchmark smoke; git diff --check
- Risk: Medium; new benchmark schema and CLI surface."
```

Expected: one implementation commit on `feature/1-issueops-quality-benchmark`.
