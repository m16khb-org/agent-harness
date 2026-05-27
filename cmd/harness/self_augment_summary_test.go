package main

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestSummarizeSelfAugmentSuccess(t *testing.T) {
	result := SelfAugmentResult{
		Runs: []SelfAugmentIteration{
			{
				Iteration: 1,
				Seed:      100,
				Steps: []StepResult{
					{Label: "fast", OK: true, DurationMS: 10},
					{Label: "slow", OK: true, DurationMS: 50},
				},
			},
			{
				Iteration: 2,
				Seed:      101,
				Steps: []StepResult{
					{Label: "fast", OK: true, DurationMS: 15},
					{Label: "slow", OK: true, DurationMS: 40},
				},
			},
		},
	}
	summary := summarizeSelfAugment(result)
	if summary.TotalRuns != 2 || summary.TotalSteps != 4 || summary.PassedSteps != 4 || summary.FailedSteps != 0 {
		t.Fatalf("unexpected summary counts: %+v", summary)
	}
	if len(summary.StepLabels) != 2 || summary.StepLabels[0] != "fast" || summary.StepLabels[1] != "slow" {
		t.Fatalf("unexpected step labels: %+v", summary.StepLabels)
	}
	if len(summary.SlowestSteps) != 4 || summary.SlowestSteps[0].Label != "slow" || summary.SlowestSteps[0].DurationMS != 50 {
		t.Fatalf("unexpected slowest steps: %+v", summary.SlowestSteps)
	}
}

func TestSummarizeSelfAugmentFailure(t *testing.T) {
	result := SelfAugmentResult{
		Runs: []SelfAugmentIteration{
			{
				Iteration: 3,
				Seed:      202,
				Steps: []StepResult{
					{Label: "go test", OK: true, DurationMS: 100},
					{Label: "MCP smoke", OK: false, DurationMS: 25, Error: "boom"},
				},
			},
		},
	}
	summary := summarizeSelfAugment(result)
	if summary.TotalRuns != 1 || summary.TotalSteps != 2 || summary.PassedSteps != 1 || summary.FailedSteps != 1 {
		t.Fatalf("unexpected summary counts: %+v", summary)
	}
	if summary.FailedIteration != 3 || summary.FailedSeed != 202 || summary.FailedStep != "MCP smoke" {
		t.Fatalf("unexpected failure pointer: %+v", summary)
	}
	if len(summary.RerunCommands) == 0 || !strings.Contains(summary.RerunCommands[len(summary.RerunCommands)-1], "--progress=jsonl") {
		t.Fatalf("expected rerun commands with progress heartbeat: %+v", summary.RerunCommands)
	}
	if summary.FailureClass != "single_failure_observation" || len(summary.FailureClusters) != 1 {
		t.Fatalf("expected single failure classification: %+v", summary)
	}
}

func TestSummarizeSelfVerificationClassifiesIntermittentFailure(t *testing.T) {
	result := SelfAugmentResult{
		Iterations: 3,
		BaseSeed:   10,
		Runs: []SelfAugmentIteration{
			{Iteration: 1, Seed: 10, Steps: []StepResult{{Label: "go test", OK: true}}},
			{Iteration: 2, Seed: 11, Steps: []StepResult{{Label: "go test", OK: false, Error: "boom"}}},
			{Iteration: 3, Seed: 12, Steps: []StepResult{{Label: "go test", OK: true}}},
		},
	}
	summary := summarizeSelfVerification(result, 95)
	if summary.FailureClass != "intermittent" {
		t.Fatalf("expected intermittent failure classification: %+v", summary)
	}
	if len(summary.FailureClusters) != 1 || summary.FailureClusters[0].Seeds[0] != 11 {
		t.Fatalf("unexpected failure clusters: %+v", summary.FailureClusters)
	}
}

func TestSelfVerificationCoverageReportsMissingLabels(t *testing.T) {
	coverage, gaps := selfVerificationCoverage([]string{"go test", "contract golden tests"})
	if len(coverage) == 0 || len(gaps) == 0 {
		t.Fatalf("expected coverage and gaps, got coverage=%+v gaps=%+v", coverage, gaps)
	}
	if !strings.Contains(gaps[0], "missing") {
		t.Fatalf("unexpected gap format: %+v", gaps)
	}
}

func TestSelfVerificationCoverageCompleteWhenAllLabelsPresent(t *testing.T) {
	labels := []string{}
	for _, definition := range selfVerificationCoverageDefinitions() {
		labels = append(labels, definition.Labels...)
	}
	coverage, gaps := selfVerificationCoverage(labels)
	if len(gaps) != 0 {
		t.Fatalf("expected no coverage gaps, got %+v", gaps)
	}
	for _, item := range coverage {
		if !item.Covered || len(item.MissingLabels) != 0 {
			t.Fatalf("unexpected uncovered item: %+v", item)
		}
	}
}

func TestSelfVerificationContractIncludesSummaryExtensions(t *testing.T) {
	contract := selfVerificationContract()
	if contract.Name != "self_verification_summary" || contract.Version < 2 || len(contract.Hash) != 64 {
		t.Fatalf("unexpected contract identity: %+v", contract)
	}
	for _, want := range []string{"goal_scores", "coverage_gaps", "slowest_steps"} {
		if !containsString(contract.RequiredFields, want) {
			t.Fatalf("contract missing required field %q: %+v", want, contract.RequiredFields)
		}
	}
	if !containsString(contract.GoalNames, "policy_security") || !containsString(contract.CoverageClaims, "secret redaction audit") {
		t.Fatalf("contract missing goals/coverage claims: %+v", contract)
	}
}

func TestSaveSelfAugmentSummary(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	result := SelfAugmentResult{
		OK:          true,
		Iterations:  10,
		BaseSeed:    300,
		ElapsedMS:   1234,
		HarnessRoot: "/tmp/harness",
		Runs: []SelfAugmentIteration{
			{
				Iteration: 1,
				Seed:      300,
				Steps: []StepResult{
					{Label: "go test", OK: true, DurationMS: 25},
				},
			},
		},
	}
	result.Summary = summarizeSelfAugment(result)
	if err := saveSelfAugmentSummary(&result, "self-verify-test"); err != nil {
		t.Fatalf("saveSelfAugmentSummary: %v", err)
	}
	if result.StateCheckpoint == nil || !result.StateCheckpoint.OK {
		t.Fatalf("missing state checkpoint: %+v", result.StateCheckpoint)
	}
	if result.StateCheckpoint.Key != "self-verify-test" || result.StateCheckpoint.Path != filepath.Join(dir, "self-verify-test.json") {
		t.Fatalf("unexpected checkpoint metadata: %+v", result.StateCheckpoint)
	}
	state, err := core.StateRead("self-verify-test")
	if err != nil {
		t.Fatalf("StateRead: %v", err)
	}
	var snapshot SelfAugmentStateSnapshot
	if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
		t.Fatalf("unmarshal saved snapshot: %v", err)
	}
	if snapshot.Kind != "self_verification_summary" || !snapshot.OK || snapshot.Summary.TotalSteps != 1 || snapshot.Summary.PassedSteps != 1 {
		t.Fatalf("unexpected saved snapshot: %+v", snapshot)
	}
}

func TestSelfVerifyProgressReporterEmitsJSONL(t *testing.T) {
	var buf bytes.Buffer
	reporter, err := newSelfVerifyProgressReporter("jsonl", &buf)
	if err != nil {
		t.Fatalf("newSelfVerifyProgressReporter: %v", err)
	}
	if reporter == nil {
		t.Fatal("expected progress reporter")
	}
	reporter.emit(SelfVerifyProgressEvent{
		Event:      "loop_start",
		LoopKind:   "self_verification",
		Iterations: 10,
		Seed:       100,
	})
	reporter.emitStepEnd("self_verification", 1, 10, 100, 1, 13, StepResult{Label: "go test", OK: true, DurationMS: 25})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("unexpected progress lines: %q", buf.String())
	}
	var event SelfVerifyProgressEvent
	if err := json.Unmarshal([]byte(lines[1]), &event); err != nil {
		t.Fatalf("progress line is not JSON: %v\n%s", err, lines[1])
	}
	if event.Event != "step_end" || event.Step != "go test" || event.OK == nil || !*event.OK || event.LastSuccess != "go test" {
		t.Fatalf("unexpected step progress event: %+v", event)
	}
}

func TestSelfVerifyProgressReporterRejectsUnknownMode(t *testing.T) {
	if reporter, err := newSelfVerifyProgressReporter("xml", io.Discard); err == nil || reporter != nil {
		t.Fatalf("expected unsupported progress mode error, got reporter=%+v err=%v", reporter, err)
	}
}

func TestTailWithBudgetMarksTruncation(t *testing.T) {
	out, truncated, original := tailWithBudget(strings.Repeat("x", 100), 40)
	if !truncated || original != 100 {
		t.Fatalf("expected truncation metadata, got truncated=%v original=%d", truncated, original)
	}
	if len(out) > 40 || !strings.Contains(out, "truncated") {
		t.Fatalf("unexpected bounded output: len=%d out=%q", len(out), out)
	}
}

func TestFindUnredactedSecretLikeFlagsRealTokens(t *testing.T) {
	findings := findUnredactedSecretLike("OPENAI_API_KEY=sk-123456789012345678901234\n")
	if len(findings) == 0 {
		t.Fatal("expected unredacted secret finding")
	}
	if !strings.Contains(findings[0], "openai_token") {
		t.Fatalf("unexpected finding: %+v", findings)
	}
}

func TestFindUnredactedSecretLikeAllowsRedactedFixtures(t *testing.T) {
	findings := findUnredactedSecretLike("TOKEN=redacted\npassword=example\n")
	if len(findings) != 0 {
		t.Fatalf("redacted fixtures should be allowed: %+v", findings)
	}
}

func TestSaveSelfAugmentLesson(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	result, err := saveSelfAugmentLesson(SelfAugmentLessonRequest{
		CandidateID: "reflexion-state-memory",
		Lesson:      "실패 교훈은 다음 cycle에서 재사용 가능해야 한다.",
		NextAction:  "다음 자가 증강 후보 선택 전에 저장된 lesson을 확인한다.",
		Source:      "unit-test",
		Severity:    "warning",
		StateKey:    "self-augment-lesson-test",
	})
	if err != nil {
		t.Fatalf("saveSelfAugmentLesson: %v", err)
	}
	if !result.OK || result.Kind != selfAugmentationLessonKind || result.StateCheckpoint == nil || !result.StateCheckpoint.OK {
		t.Fatalf("unexpected lesson result: %+v", result)
	}
	if result.LLMWikiDraft.Title == "" || result.LLMWikiDraft.Content == "" || result.LLMWikiDraft.Type != "session" {
		t.Fatalf("missing llm-wiki draft: %+v", result.LLMWikiDraft)
	}
	state, err := core.StateRead("self-augment-lesson-test")
	if err != nil {
		t.Fatalf("StateRead: %v", err)
	}
	var snapshot SelfAugmentLessonStateSnapshot
	if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
		t.Fatalf("unmarshal saved lesson snapshot: %v", err)
	}
	if snapshot.Kind != selfAugmentationLessonKind || snapshot.CandidateID != "reflexion-state-memory" || snapshot.NextAction == "" {
		t.Fatalf("unexpected lesson snapshot: %+v", snapshot)
	}
}

func TestSaveSelfAugmentPlan(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	result := planSelfAugmentation(SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95})
	if err := saveSelfAugmentPlan(&result, "self-augment-plan-test"); err != nil {
		t.Fatalf("saveSelfAugmentPlan: %v", err)
	}
	if result.StateCheckpoint == nil || !result.StateCheckpoint.OK {
		t.Fatalf("missing plan checkpoint: %+v", result.StateCheckpoint)
	}
	if result.StateCheckpoint.Key != "self-augment-plan-test" || result.StateCheckpoint.Path != filepath.Join(dir, "self-augment-plan-test.json") {
		t.Fatalf("unexpected plan checkpoint metadata: %+v", result.StateCheckpoint)
	}
	state, err := core.StateRead("self-augment-plan-test")
	if err != nil {
		t.Fatalf("StateRead: %v", err)
	}
	var snapshot SelfAugmentPlanStateSnapshot
	if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
		t.Fatalf("unmarshal saved plan snapshot: %v", err)
	}
	if snapshot.Kind != selfAugmentationPlanKind || snapshot.LoopKind != "self_augmentation" {
		t.Fatalf("unexpected saved plan snapshot: %+v", snapshot)
	}
	if snapshot.CandidateCount < 10 || len(snapshot.SatisfiedCandidateIDs) == 0 {
		t.Fatalf("saved plan did not preserve candidate memory: %+v", snapshot)
	}
}

func TestCompareSelfAugmentSummaries(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	baseSummary := SelfAugmentSummary{
		TotalRuns:   10,
		TotalSteps:  20,
		PassedSteps: 20,
		StepLabels:  []string{"go test", "MCP smoke"},
		SlowestSteps: []SelfAugmentSlowStep{
			{Iteration: 1, Seed: 400, Label: "go test", DurationMS: 1000},
		},
	}
	candidateSummary := baseSummary
	if err := writeSelfAugmentSnapshotRecord(dir, "baseline", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      400,
		ElapsedMS:     1000,
		HarnessRoot:   "/tmp/harness",
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       baseSummary,
	}); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "candidate", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      400,
		ElapsedMS:     1100,
		HarnessRoot:   "/tmp/harness",
		GeneratedAt:   "2000-01-01T00:01:00Z",
		Summary:       candidateSummary,
	}); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	okResult, err := compareSelfAugmentSummaries("baseline", "candidate", 20)
	if err != nil {
		t.Fatalf("compare ok: %v", err)
	}
	if !okResult.OK || okResult.Regressed || okResult.ElapsedDeltaMS != 100 || okResult.FailedStepsDelta != 0 {
		t.Fatalf("unexpected non-regression result: %+v", okResult)
	}
	regressed, err := compareSelfAugmentSummaries("baseline", "candidate", 5)
	if err != nil {
		t.Fatalf("compare regression: %v", err)
	}
	if !regressed.OK || !regressed.Regressed || len(regressed.Regressions) == 0 {
		t.Fatalf("expected regression: %+v", regressed)
	}
}

func TestCompareSelfAugmentSummariesDetectsFailedStepRegression(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	baseline := SelfAugmentSummary{TotalRuns: 10, TotalSteps: 20, PassedSteps: 20, StepLabels: []string{"go test", "MCP smoke"}}
	candidate := SelfAugmentSummary{TotalRuns: 10, TotalSteps: 20, PassedSteps: 19, FailedSteps: 1, StepLabels: []string{"go test"}}
	if err := writeSelfAugmentSnapshotRecord(dir, "baseline", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      500,
		ElapsedMS:     1000,
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       baseline,
	}); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "candidate", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            false,
		Iterations:    10,
		BaseSeed:      500,
		ElapsedMS:     900,
		GeneratedAt:   "2000-01-01T00:01:00Z",
		Summary:       candidate,
	}); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	result, err := compareSelfAugmentSummaries("baseline", "candidate", 20)
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if !result.Regressed || !containsString(result.MissingStepLabels, "MCP smoke") || result.FailedStepsDelta != 1 {
		t.Fatalf("expected failed-step and missing-label regression: %+v", result)
	}
}

func TestCompareSelfAugmentSummariesDetectsSlowStepRegression(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	baseline := SelfAugmentSummary{
		TotalRuns:   10,
		TotalSteps:  20,
		PassedSteps: 20,
		StepLabels:  []string{"go test", "MCP smoke"},
		SlowestSteps: []SelfAugmentSlowStep{
			{Iteration: 1, Seed: 600, Label: "go test", DurationMS: 1000},
			{Iteration: 1, Seed: 600, Label: "MCP smoke", DurationMS: 100},
		},
	}
	candidate := baseline
	candidate.SlowestSteps = []SelfAugmentSlowStep{
		{Iteration: 1, Seed: 600, Label: "go test", DurationMS: 1400},
		{Iteration: 1, Seed: 600, Label: "MCP smoke", DurationMS: 100},
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "baseline", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      600,
		ElapsedMS:     1000,
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       baseline,
	}); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "candidate", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      600,
		ElapsedMS:     1000,
		GeneratedAt:   "2000-01-01T00:01:00Z",
		Summary:       candidate,
	}); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	result, err := compareSelfAugmentSummaries("baseline", "candidate", 20)
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	if !result.Regressed || len(result.SlowStepRegressions) != 1 {
		t.Fatalf("expected slow-step regression: %+v", result)
	}
	regression := result.SlowStepRegressions[0]
	if regression.Label != "go test" || regression.DeltaMS != 400 || regression.DeltaPct != 40 {
		t.Fatalf("unexpected slow-step regression detail: %+v", regression)
	}
	if !containsString(result.Regressions, "slow_step:go test_increased_by_40.00_pct") {
		t.Fatalf("missing slow-step regression marker: %+v", result.Regressions)
	}
}

func TestLintMermaidBlocksEnforcesGeniusThinkRules(t *testing.T) {
	good := "```mermaid\nflowchart LR\n    A[\"한글 노드<br/>설명\"] --> B[\"Next\"]\n    subgraph \"계획 레이어\"\n    end\n```\n"
	if issues := lintMermaidBlocks("good.md", good); len(issues) != 0 {
		t.Fatalf("valid mermaid was rejected: %+v", issues)
	}

	bad := "```mermaid\nflowchart LR\n    A[한글 노드<br>설명] --> B[Next]\n    subgraph 계획 레이어\n    end\n```\n"
	issues := lintMermaidBlocks("bad.md", bad)
	for _, want := range []string{"bad.md:3 mermaid uses <br>; use <br/>", "bad.md:3 mermaid node text must start with a quote", "bad.md:4 mermaid subgraph title must be quoted"} {
		if !containsString(issues, want) {
			t.Fatalf("missing %q in issues: %+v", want, issues)
		}
	}

	documentedBadExample := "## 잘못된 예시 (파싱 에러 발생)\n\n```mermaid\nflowchart LR\n    A[한글 노드<br>설명]\n```\n"
	if issues := lintMermaidBlocks("genius.md", documentedBadExample); len(issues) != 0 {
		t.Fatalf("documented bad example should be ignored: %+v", issues)
	}
}

func TestPromoteSelfAugmentBaseline(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	summary := SelfAugmentSummary{TotalRuns: 10, TotalSteps: 20, PassedSteps: 20, StepLabels: []string{"go test"}}
	source := SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      700,
		ElapsedMS:     1000,
		HarnessRoot:   "/tmp/harness",
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       summary,
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "candidate", source); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	dry, err := promoteSelfAugmentBaseline("candidate", "baseline", false)
	if err != nil {
		t.Fatalf("promote dry-run: %v", err)
	}
	if !dry.OK || !dry.DryRun || dry.Promoted {
		t.Fatalf("unexpected dry-run promote result: %+v", dry)
	}
	if _, err := core.StateRead("baseline"); err == nil {
		t.Fatalf("dry-run wrote baseline")
	}
	confirmed, err := promoteSelfAugmentBaseline("candidate", "baseline", true)
	if err != nil {
		t.Fatalf("promote confirm: %v", err)
	}
	if !confirmed.OK || confirmed.DryRun || !confirmed.Promoted || confirmed.Path != filepath.Join(dir, "baseline.json") {
		t.Fatalf("unexpected confirmed promote result: %+v", confirmed)
	}
	baseline, err := readSelfAugmentStateSnapshot("baseline")
	if err != nil {
		t.Fatalf("read promoted baseline: %v", err)
	}
	if baseline.GeneratedAt != source.GeneratedAt || baseline.Summary.TotalSteps != source.Summary.TotalSteps {
		t.Fatalf("promoted baseline drifted: %+v", baseline)
	}
	compared, err := compareSelfAugmentSummaries("baseline", "candidate", 0)
	if err != nil {
		t.Fatalf("compare promoted: %v", err)
	}
	if compared.Regressed || compared.ElapsedDeltaMS != 0 {
		t.Fatalf("promoted baseline should compare cleanly: %+v", compared)
	}
}

func TestSelfAugmentHistory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", dir)
	summary := SelfAugmentSummary{
		TotalRuns:   10,
		TotalSteps:  20,
		PassedSteps: 20,
		StepLabels:  []string{"go test", "MCP smoke"},
		SlowestSteps: []SelfAugmentSlowStep{
			{Iteration: 1, Seed: 800, Label: "go test", DurationMS: 1000},
		},
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "self-verify-old", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      800,
		ElapsedMS:     1200,
		GeneratedAt:   "2000-01-01T00:00:00Z",
		Summary:       summary,
	}); err != nil {
		t.Fatalf("write old snapshot: %v", err)
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "self-verify-new", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      801,
		ElapsedMS:     1000,
		GeneratedAt:   "2000-01-02T00:00:00Z",
		Summary:       summary,
	}); err != nil {
		t.Fatalf("write new snapshot: %v", err)
	}
	if err := writeSelfAugmentSnapshotRecord(dir, "other-summary", SelfAugmentStateSnapshot{
		SchemaVersion: 1,
		Kind:          "self_verification_summary",
		OK:            true,
		Iterations:    10,
		BaseSeed:      802,
		ElapsedMS:     900,
		GeneratedAt:   "2000-01-03T00:00:00Z",
		Summary:       summary,
	}); err != nil {
		t.Fatalf("write other snapshot: %v", err)
	}
	if _, err := core.StateWrite("self-verify-note", "not a summary"); err != nil {
		t.Fatalf("write non-summary state: %v", err)
	}

	limited, err := selfAugmentHistory("self-verify", 1)
	if err != nil {
		t.Fatalf("history limited: %v", err)
	}
	if !limited.OK || limited.TotalMatches != 2 || limited.Returned != 1 || limited.Entries[0].Key != "self-verify-new" {
		t.Fatalf("unexpected limited history: %+v", limited)
	}
	if !historySkippedKey(limited.Skipped, "self-verify-note") {
		t.Fatalf("expected non-summary key to be skipped: %+v", limited.Skipped)
	}

	all, err := selfAugmentHistory("", 0)
	if err != nil {
		t.Fatalf("history all: %v", err)
	}
	if all.TotalMatches != 3 || all.Returned != 3 || all.Entries[0].Key != "other-summary" {
		t.Fatalf("unexpected all history ordering/counts: %+v", all)
	}
}

func TestPlanRiskQATierFromPaths(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		tier     string
		commands []string
	}{
		{
			name:     "no changes",
			paths:    nil,
			tier:     "standard",
			commands: []string{},
		},
		{
			name:     "docs only",
			paths:    []string{"agent_docs/TESTING.md"},
			tier:     "standard",
			commands: []string{},
		},
		{
			name:     "go but not sensitive",
			paths:    []string{"examples/demo.go"},
			tier:     "static",
			commands: []string{"go vet ./..."},
		},
		{
			name:     "sensitive go",
			paths:    []string{"internal/core/policy.go", "cmd/harness/main.go"},
			tier:     "elevated",
			commands: []string{"go test -race ./... -count=1", "go vet ./..."},
		},
	}
	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			plan := planRiskQATierFromPaths(tc.paths)
			if plan.Tier != tc.tier {
				t.Fatalf("tier=%q want %q: %+v", plan.Tier, tc.tier, plan)
			}
			if !sameStringSlice(plan.Commands, tc.commands) {
				t.Fatalf("commands=%+v want %+v", plan.Commands, tc.commands)
			}
		})
	}
}

func TestParseGitStatusPath(t *testing.T) {
	tests := map[string]string{
		" M cmd/harness/main.go":               "cmd/harness/main.go",
		"?? internal/adapter/new_test.go":      "internal/adapter/new_test.go",
		"R  old/path.go -> internal/core/x.go": "internal/core/x.go",
		"":                                     "",
	}
	for line, want := range tests {
		if got := parseGitStatusPath(line); got != want {
			t.Fatalf("parseGitStatusPath(%q)=%q want %q", line, got, want)
		}
	}
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPlanSelfAugmentationUsesGeniusThinkAndScoreGate(t *testing.T) {
	result := planSelfAugmentation(SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95})
	if !result.OK || result.LoopKind != "self_augmentation" || result.KoreanName != selfAugmentationKoreanName {
		t.Fatalf("unexpected loop identity: %+v", result)
	}
	if !result.UsesGeniusThink || len(result.SelectedFormulas) < 2 {
		t.Fatalf("expected GENIUS_THINK formulas: %+v", result.SelectedFormulas)
	}
	if len(result.Candidates) < 10 {
		t.Fatalf("expected candidate curriculum: %+v", result.Candidates)
	}
	if result.SelectedCandidate != nil && result.SelectedCandidate.Status != selfAugmentCandidateStatusOpen {
		t.Fatalf("selected candidate must be an open improvement, got %+v", result.SelectedCandidate)
	}
	if candidateByID(result.Candidates, "loop-taxonomy-score-gates").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("completed taxonomy candidate should be kept for audit but skipped for selection: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "durable-augmentation-memory").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("durable memory candidate should be satisfied after state capture support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "reflexion-state-memory").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("reflexion memory candidate should be satisfied after lesson capture support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "adapter-contract-matrix").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("adapter contract matrix candidate should be satisfied after matrix golden support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "qa-race-tier").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("QA race tier candidate should be satisfied after risk-tier QA support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "repo-local-augmentation-sandbox").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("repo-local sandbox candidate should be satisfied after path boundary hardening: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "performance-baseline").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("performance baseline candidate should be satisfied after slow-step compare support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "genius-mermaid-lint").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("Mermaid lint candidate should be satisfied after QA lint support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "install-dry-run-mode").Status != selfAugmentCandidateStatusSatisfied {
		t.Fatalf("install dry-run candidate should be satisfied after dry-run planning support: %+v", result.Candidates)
	}
	if result.SelectedCandidate == nil {
		for _, candidate := range result.Candidates {
			if candidate.Status == selfAugmentCandidateStatusOpen {
				t.Fatalf("nil selected candidate is valid only when no open candidates remain: %+v", result.Candidates)
			}
		}
	}
	if result.TerminationEligible {
		t.Fatalf("planner must not claim implementation termination before a diff is applied")
	}
}

func candidateByID(candidates []SelfAugmentCandidate, id string) SelfAugmentCandidate {
	for _, candidate := range candidates {
		if candidate.ID == id {
			return candidate
		}
	}
	return SelfAugmentCandidate{}
}

func historySkippedKey(skipped []SelfAugmentHistorySkipped, key string) bool {
	for _, item := range skipped {
		if item.Key == key {
			return true
		}
	}
	return false
}
