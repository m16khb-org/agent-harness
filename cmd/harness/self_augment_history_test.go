package main

import (
	"path/filepath"
	"testing"

	"agent-harness/internal/core"
)

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
	if limited.Retention != nil {
		t.Fatalf("retention should be omitted when no retention limit is requested: %+v", limited.Retention)
	}

	retentionPlan, err := selfAugmentHistory("self-verify", 0, selfAugmentHistoryRetentionOptions{Limit: 1})
	if err != nil {
		t.Fatalf("history retention plan: %v", err)
	}
	if retentionPlan.Retention == nil || !retentionPlan.Retention.Enabled || retentionPlan.Retention.Limit != 1 {
		t.Fatalf("retention plan missing: %+v", retentionPlan.Retention)
	}
	if retentionPlan.Retention.TotalMatches != 2 ||
		!containsString(retentionPlan.Retention.RetainedKeys, "self-verify-new") ||
		!containsString(retentionPlan.Retention.CandidateKeys, "self-verify-old") ||
		len(retentionPlan.Retention.DeletedKeys) != 0 {
		t.Fatalf("unexpected retention plan: %+v", retentionPlan.Retention)
	}
	if !containsString(retentionPlan.Warnings, "history_retention_candidates:1") {
		t.Fatalf("retention plan should warn about prune candidates: %+v", retentionPlan.Warnings)
	}

	retentionDryRun, err := selfAugmentHistory("self-verify", 0, selfAugmentHistoryRetentionOptions{Limit: 1, PruneRequested: true})
	if err != nil {
		t.Fatalf("history retention dry-run: %v", err)
	}
	if retentionDryRun.Retention == nil || !retentionDryRun.Retention.DryRun || retentionDryRun.Retention.Confirm || len(retentionDryRun.Retention.DeletedKeys) != 0 {
		t.Fatalf("unexpected retention dry-run: %+v", retentionDryRun.Retention)
	}
	if _, err := core.StateRead("self-verify-old"); err != nil {
		t.Fatalf("retention dry-run deleted old summary: %v", err)
	}

	retentionConfirmed, err := selfAugmentHistory("self-verify", 0, selfAugmentHistoryRetentionOptions{Limit: 1, PruneRequested: true, Confirm: true})
	if err != nil {
		t.Fatalf("history retention confirm: %v", err)
	}
	if retentionConfirmed.Retention == nil || retentionConfirmed.Retention.DryRun || !retentionConfirmed.Retention.Confirm || !containsString(retentionConfirmed.Retention.DeletedKeys, "self-verify-old") {
		t.Fatalf("unexpected retention confirm: %+v", retentionConfirmed.Retention)
	}
	if _, err := core.StateRead("self-verify-old"); err == nil {
		t.Fatalf("retention confirm left old summary in state")
	}

	all, err := selfAugmentHistory("", 0)
	if err != nil {
		t.Fatalf("history all: %v", err)
	}
	if all.TotalMatches != 2 || all.Returned != 2 || all.Entries[0].Key != "other-summary" {
		t.Fatalf("unexpected all history ordering/counts: %+v", all)
	}
}

func TestSelfAugmentHistoryRetentionRejectsUnsafeOptions(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if _, err := selfAugmentHistory("self-verify", 0, selfAugmentHistoryRetentionOptions{Limit: -1}); err == nil {
		t.Fatalf("negative retention limit was accepted")
	}
	if _, err := selfAugmentHistory("self-verify", 0, selfAugmentHistoryRetentionOptions{Confirm: true}); err == nil {
		t.Fatalf("confirm without prune-retention was accepted")
	}
	if _, err := selfAugmentHistory("self-verify", 0, selfAugmentHistoryRetentionOptions{PruneRequested: true}); err == nil {
		t.Fatalf("prune-retention without positive retention limit was accepted")
	}
}

func historySkippedKey(skipped []SelfAugmentHistorySkipped, key string) bool {
	for _, item := range skipped {
		if item.Key == key {
			return true
		}
	}
	return false
}
