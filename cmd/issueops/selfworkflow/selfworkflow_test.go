package selfworkflow

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"issueops/cmd/issueops/selfworkflow/llmeval"
)

func TestSelfWorkflowCandidateExportAndStateWrappers(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root := t.TempDir()
	candidatePath := filepath.Join(root, "skills", "self-verify", "CANDIDATES.md")
	if err := os.MkdirAll(filepath.Dir(candidatePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(candidatePath, []byte("# Candidates\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	restore := IssueOpsRoot
	IssueOpsRoot = func() string { return root }
	t.Cleanup(func() { IssueOpsRoot = restore })

	result := ExportSelfVerificationCandidates()
	if !result.OK || !result.SourceExists || result.IssueOpsRoot != root {
		t.Fatalf("unexpected export result: %#v", result)
	}
	if result.CandidateCount == 0 || len(result.Candidates) != result.CandidateCount {
		t.Fatalf("candidate catalog mismatch: %#v", result)
	}
	if SelectedSelfVerificationCandidateID(nil) != "none" {
		t.Fatal("nil verification candidate should format as none")
	}
	if result.SelectedCandidate != nil && SelectedSelfVerificationCandidateID(result.SelectedCandidate) == "none" {
		t.Fatal("selected candidate should have a real ID")
	}
	openIDs := SelfVerificationCandidateIDsByStatus(result.Candidates, SelfAugmentCandidateStatusOpen)
	if len(openIDs) != len(result.OpenCandidateIDs) {
		t.Fatalf("open ID count mismatch: %d vs %d", len(openIDs), len(result.OpenCandidateIDs))
	}
	if err := SaveSelfVerificationCandidateExport(&result, "verify-export"); err != nil {
		t.Fatalf("SaveSelfVerificationCandidateExport: %v", err)
	}
}

func TestSelfWorkflowAugmentWrapperHelpers(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, ".issueops")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("coverage quality"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "TESTING.md"), []byte("self-verify coverage quality"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "signal.go"), []byte("package docs\nconst qualitySignal = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if !DocsContainTerm(root, "coverage") || !DirContainsTerm(root, ".issueops", "qualitySignal") || !FileContainsTerm(root, ".issueops/TESTING.md", "self-verify") {
		t.Fatal("term detection wrappers should find fixture content")
	}
	if FileContainsTerm(root, "missing.md", "x") {
		t.Fatal("missing file should not contain term")
	}
	if ScoreBool(true) != 100 || ScoreBool(false) != 0 {
		t.Fatal("ScoreBool wrapper returned unexpected values")
	}
	if FormatScore(98.25) == "" {
		t.Fatal("FormatScore should return non-empty text")
	}
	if SelectedCandidateID(nil) != "" {
		t.Fatal("nil augment candidate should format as empty ID")
	}
	candidate := SelfAugmentCandidate{ID: "candidate-refill-curriculum", Impact: 1, Feasibility: 1, Novelty: 1}
	MarkSatisfiedSelfAugmentCandidate(&candidate, SelfAugmentRepoSignals{HasCandidateRefill: true})
	if candidate.Status != SelfAugmentCandidateStatusSatisfied {
		t.Fatalf("candidate status = %q", candidate.Status)
	}
	candidates := SelfAugmentCandidates(SelfAugmentRepoSignals{HasCandidateRefill: true})
	if len(candidates) == 0 || SelfAugmentCandidateScore(candidates[0]) <= 0 {
		t.Fatal("expected scored augment candidates")
	}
	ids := SelfAugmentCandidateIDsByStatus(candidates, SelfAugmentCandidateStatusOpen)
	if len(ids) == 0 {
		t.Fatal("expected open augment candidate IDs")
	}
	if len(SelfAugmentResearchInfluences()) == 0 {
		t.Fatal("research influences should not be empty")
	}
	if !AllSelfAugmentGoalsPassed([]SelfAugmentGoal{{Passed: true}}) || AllSelfAugmentGoalsPassed([]SelfAugmentGoal{{Passed: false}}) {
		t.Fatal("goal pass wrapper returned unexpected value")
	}
	signals := CollectSelfAugmentRepoSignals(root, 1, []string{"self-augment"}, "GENIUS_THINK")
	if !signals.HasGeniusThink || signals.DocsIndexed != 1 {
		t.Fatalf("unexpected repo signals: %#v", signals)
	}
}

func TestSelfWorkflowSummaryAndHistoryWrappers(t *testing.T) {
	if !containsString([]string{"a", "b"}, "b") || containsString([]string{"a"}, "z") {
		t.Fatal("containsString mismatch")
	}
	if fileExists(filepath.Join(t.TempDir(), "missing")) {
		t.Fatal("missing file should not exist")
	}
	if _, ok := ParseSelfAugmentTimestamp(time.Now().UTC().Format(time.RFC3339Nano)); !ok {
		t.Fatal("timestamp should parse")
	}
	if _, ok := ParseSelfAugmentTimestamp("bad"); ok {
		t.Fatal("bad timestamp should not parse")
	}
	if got := MissingStrings([]string{"a", "b"}, []string{"b"}); len(got) != 1 || got[0] != "a" {
		t.Fatalf("MissingStrings = %#v", got)
	}
	if NonNilStringSlice(nil) == nil || NonNilSlowStepSlice(nil) == nil {
		t.Fatal("non-nil wrappers should return empty slices")
	}
	slow := []SelfAugmentSlowStep{{Label: "test", DurationMS: 10}, {Label: "test", DurationMS: 20}}
	if MaxSlowStepDurationByLabel(slow)["test"] != 20 {
		t.Fatal("max slow step duration mismatch")
	}
	stats := BuildStepDurationStats(map[string][]int64{"test": {10, 20, 30}})
	if len(stats) != 1 || stats[0].Label != "test" {
		t.Fatalf("BuildStepDurationStats = %#v", stats)
	}
	if StepDurationStatByLabel(stats)["test"].MaxDurationMS != 30 {
		t.Fatal("step duration lookup mismatch")
	}
	regressions := CompareStepBudgetRegressions(
		[]SelfAugmentStepDurationStat{{Label: "test", Count: 1, P95DurationMS: 100}},
		[]SelfAugmentStepDurationStat{{Label: "test", Count: 1, P95DurationMS: 140}},
		10,
	)
	if len(regressions) != 1 {
		t.Fatalf("expected budget regression, got %#v", regressions)
	}
	slowRegressions := CompareSlowestStepRegressions(
		[]SelfAugmentSlowStep{{Label: "test", DurationMS: 10}},
		[]SelfAugmentSlowStep{{Label: "test", DurationMS: 20}},
		10,
	)
	if len(slowRegressions) != 1 {
		t.Fatalf("expected slow-step regression, got %#v", slowRegressions)
	}
	result := NewSelfVerifyLoopResult(1, 100, 95)
	if result.Iterations != 1 || result.IssueOpsRoot == "" {
		t.Fatalf("unexpected loop result: %#v", result)
	}
	coverage, gaps := BuildSelfVerificationCoverage([]string{"install dry-run"})
	if len(coverage) == 0 || len(gaps) == 0 {
		t.Fatal("coverage definitions should produce coverage and gaps")
	}
	if len(SelfVerificationCoverageDefinitions()) == 0 || len(SelfVerificationGoalDefinitions()) == 0 {
		t.Fatal("self-verification definitions should not be empty")
	}
	if BuildSelfVerificationContract().Name == "" {
		t.Fatal("contract should have a name")
	}
	if len(SelfVerifyRerunCommands("go test", 100, 95)) == 0 {
		t.Fatal("rerun commands should not be empty")
	}
	if _, ok := SelfVerifyStepRerunCommand("unknown"); ok {
		t.Fatal("unknown step should not have a rerun command")
	}
	summary := SummarizeSelfVerification(SelfAugmentResult{Runs: []SelfAugmentIteration{{Steps: []StepResult{{Label: "step", OK: true}}}}}, 95)
	if summary.TotalSteps != 1 {
		t.Fatalf("summary total steps = %d", summary.TotalSteps)
	}
	if len(SelfVerificationFailureClusters(SelfAugmentResult{Runs: []SelfAugmentIteration{{Seed: 1, Steps: []StepResult{{Label: "fail", OK: false}}}}})) != 1 {
		t.Fatal("failure clusters should include failed step")
	}
}

func TestSelfVerifyLLMEvalAndProgressWrappers(t *testing.T) {
	if NormalizeSelfVerifyLLMEvalMode("") != "advisory" {
		t.Fatal("empty LLM eval mode should normalize to advisory")
	}
	if err := ValidateSelfVerifyLLMEvalMode("gate"); err != nil {
		t.Fatalf("valid gate mode rejected: %v", err)
	}
	if err := ValidateSelfVerifyLLMEvalMode("bad"); err == nil {
		t.Fatal("invalid LLM eval mode should fail")
	}
	enabled, mode, err := ParseSelfVerifyLLMEvalEnv("gate")
	if err != nil || !enabled || mode != "gate" {
		t.Fatalf("ParseSelfVerifyLLMEvalEnv = enabled=%v mode=%q err=%v", enabled, mode, err)
	}
	config, err := ResolveSelfVerifyLLMEvalConfig(false, false, "", false, func(string) (string, bool) { return "advisory", true })
	if err != nil || !config.Enabled || config.Mode != "advisory" {
		t.Fatalf("ResolveSelfVerifyLLMEvalConfig = %#v err=%v", config, err)
	}
	var eval SelfVerifyLLMEvalResult
	raw := []byte(`prefix {"ok":true,"mode":"gate","execution_class":"foreground_blocking","read_only":true,"score":99,"summary":"ok","evidence_packet_bytes":10}`)
	if err := DecodeSelfVerifyLLMEval(raw, &eval); err != nil {
		t.Fatalf("DecodeSelfVerifyLLMEval: %v", err)
	}
	if err := DecodeSelfVerifyLLMEvalStrict([]byte(`{"ok":true,"mode":"advisory","execution_class":"foreground_blocking","read_only":true,"score":99,"evidence_packet_bytes":10}`), &eval); err != nil {
		t.Fatalf("DecodeSelfVerifyLLMEvalStrict: %v", err)
	}
	if extracted, ok := ExtractSelfVerifyLLMEvalJSON(raw); !ok || len(extracted) == 0 {
		t.Fatal("expected embedded JSON extraction")
	}
	if BoundedLLMEvalError("prefix", errors.New("boom"), "output") == "" {
		t.Fatal("bounded error should be non-empty")
	}
	result := SelfAugmentResult{OK: true, TerminationEligible: true, Summary: SelfAugmentSummary{TerminationEligible: true}}
	if out, err := ApplySelfVerifyLLMEval(result, SelfVerifyLLMEvalOptions{}); err != nil || !out.OK {
		t.Fatalf("disabled LLM eval should preserve result: %#v err=%v", out, err)
	}
	result.LLMEval = &SelfVerifyLLMEvalResult{OK: false, Mode: "gate", Score: 50, Blockers: []string{"risk"}}
	if out, err := ApplySelfVerifyLLMGate(result, 95); err == nil || out.OK || out.TerminationEligible {
		t.Fatalf("gate should fail closed: %#v err=%v", out, err)
	}
	if SelfVerifyLLMResponseSchemaExample() == "" || len(SelfVerifyLLMResponseFieldTypes()) == 0 {
		t.Fatal("LLM response schema helpers should return content")
	}
	if boolPtr(true) == nil || !*boolPtr(true) {
		t.Fatal("boolPtr should return pointer to value")
	}
	var progress bytes.Buffer
	reporter, err := NewSelfVerifyProgressReporter("jsonl", &progress)
	if err != nil {
		t.Fatalf("NewSelfVerifyProgressReporter: %v", err)
	}
	EmitSelfVerifyLoopStart(reporter, "self_verification", 1, 100)
	EmitSelfVerifyLoopEnd(reporter, "self_verification", 1, 100, true, "")
	if progress.Len() == 0 {
		t.Fatal("progress reporter should emit JSONL")
	}
}

func TestSelfVerifyCLIAndStateWrappers(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	runResult := SelfAugmentResult{
		OK:                  true,
		LoopKind:            "self_verification",
		KoreanName:          SelfVerificationKoreanName,
		Iterations:          1,
		BaseSeed:            100,
		TargetScore:         95,
		TerminationEligible: true,
		IssueOpsRoot:        t.TempDir(),
		Summary:             SelfAugmentSummary{MinimumGoalScore: 100, TerminationEligible: true},
	}
	if err := RunSelfVerifyWithDeps([]string{"--json", "--save-state", "--state-key", "verify-latest", "--seed", "100"}, SelfVerifyRunDeps{
		LookupEnv:      func(string) (string, bool) { return "", false },
		ProgressWriter: &bytes.Buffer{},
		Verify: func(request SelfVerifyRequest) (SelfAugmentResult, error) {
			if request.BaseSeed != 100 || request.Verbose {
				t.Fatalf("unexpected verify request: %+v", request)
			}
			return runResult, nil
		},
		SaveSummary: func(result *SelfAugmentResult, key string) error {
			if key != "verify-latest" {
				t.Fatalf("state key = %q", key)
			}
			return nil
		},
		ApplyLLMEval: func(SelfAugmentResult, llmeval.SelfVerifyLLMEvalOptions) (SelfAugmentResult, error) {
			return SelfAugmentResult{}, fmt.Errorf("LLM eval should not run without an explicit flag or env")
		},
		PrintJSON: func(any) error { return nil },
	}); err != nil {
		t.Fatalf("RunSelfVerifyWithDeps: %v", err)
	}
	candidateExport := SelfVerificationCandidateExportResult{OK: true, Kind: SelfVerificationCandidateExportKind, KoreanName: SelfVerificationKoreanName, CandidateCount: 1, Candidates: []SelfVerificationCandidate{{ID: "c1"}}}
	savedCandidate := false
	if err := RunSelfVerifyCandidatesWithDeps([]string{"--save-state", "--state-key", "candidates", "--json"}, SelfVerifyCandidatesDeps{
		Export: func() SelfVerificationCandidateExportResult { return candidateExport },
		Save: func(result *SelfVerificationCandidateExportResult, key string) error {
			savedCandidate = key == "candidates" && result.CandidateCount == 1
			return nil
		},
	}); err != nil {
		t.Fatalf("RunSelfVerifyCandidatesWithDeps: %v", err)
	}
	if !savedCandidate {
		t.Fatal("candidate export was not saved through deps")
	}
	if err := RunSelfVerifyPromoteWithDeps([]string{"--from-key", "verify-latest", "--baseline-key", "baseline", "--json"}, SelfVerifyPromoteDeps{
		Promote: func(fromKey, baselineKey string, confirm, allowFailedSource bool) (SelfAugmentPromoteResult, error) {
			return SelfAugmentPromoteResult{OK: true, FromKey: fromKey, BaselineKey: baselineKey, Confirm: confirm, DryRun: !confirm, SourcePassed: true}, nil
		},
	}); err != nil {
		t.Fatalf("RunSelfVerifyPromoteWithDeps: %v", err)
	}
	if !IsSelfVerificationSummaryKind(SelfVerificationSummaryKind) || IsSelfVerificationSummaryKind("other") {
		t.Fatal("summary kind classifier mismatch")
	}
	snapshot := NewSelfVerificationSummarySnapshot(runResult, time.Now().UTC())
	if snapshot.Kind != SelfVerificationSummaryKind {
		t.Fatalf("snapshot kind = %q", snapshot.Kind)
	}
	if err := WriteSelfAugmentSnapshotRecord(t.TempDir(), "snapshot", snapshot); err != nil {
		t.Fatalf("WriteSelfAugmentSnapshotRecord: %v", err)
	}
	if err := SaveSelfVerificationSummary(&runResult, "verify-summary"); err != nil {
		t.Fatalf("SaveSelfVerificationSummary: %v", err)
	}
	if _, err := ReadSelfAugmentStateSnapshot("verify-summary"); err != nil {
		t.Fatalf("ReadSelfAugmentStateSnapshot: %v", err)
	}
	if err := SaveSelfAugmentSummary(&runResult, "augment-summary"); err != nil {
		t.Fatalf("SaveSelfAugmentSummary: %v", err)
	}
	plan := PlanSelfAugmentation(SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95})
	if !plan.OK || plan.Cycles != 1 {
		t.Fatalf("PlanSelfAugmentation: %#v", plan)
	}
	if err := SaveSelfAugmentPlan(&plan, "augment-plan"); err != nil {
		t.Fatalf("SaveSelfAugmentPlan: %v", err)
	}
	lesson, err := SaveSelfAugmentLesson(SelfAugmentLessonRequest{CandidateID: "c1", Lesson: "lesson", NextAction: "next", Source: "test", Severity: "low", StateKey: "lesson"})
	if err != nil || !lesson.OK {
		t.Fatalf("SaveSelfAugmentLesson: %#v err=%v", lesson, err)
	}
	if StateKeySlug("Hello, World!") != "hello-world" {
		t.Fatalf("StateKeySlug mismatch")
	}
	promote, err := PromoteSelfAugmentBaseline("verify-summary", "baseline", false, false)
	if err != nil || !promote.DryRun {
		t.Fatalf("PromoteSelfAugmentBaseline dry-run: %#v err=%v", promote, err)
	}
}
