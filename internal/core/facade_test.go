package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPolicyStateUtilityAndProjectDocFacades(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	repo := t.TempDir()
	writeCoreTestFile(t, repo, "go.mod", "module example.com/repo\n")
	writeCoreTestFile(t, repo, "README.md", "# Repo\n\n## Usage\n")
	writeCoreTestFile(t, repo, filepath.Join(ProjectDocsDir, "TESTING.md"), "# Testing\n")

	eval := EvaluateCommandPolicy(CommandPolicyRequest{WorkspaceRoot: repo, CWD: repo, Argv: []string{"git", "status", "--short"}})
	if eval.Tier.Name != PolicyTierReadOnly {
		t.Fatalf("unexpected policy eval: %#v", eval)
	}
	if FakeRunCommand(CommandPolicyRequest{WorkspaceRoot: repo, CWD: repo, Argv: []string{"git", "status"}}).Policy.Tier.Name != PolicyTierReadOnly {
		t.Fatal("fake run should evaluate read-only command")
	}
	if len(CommandPolicySummary()) == 0 || IsPolicyDenied(errors.New("x")) {
		t.Fatal("unexpected policy summary/denied result")
	}
	if cleanEnvAllowlist([]string{"B", "", "A", "A"}) == nil {
		t.Fatal("env allowlist should return a cleaned slice")
	}
	_ = redactArgv([]string{"--token=secret"})
	_ = redactFreeform("api_key=secret")
	if len(uniqSorted([]string{"b", "a", "b", ""})) != 2 {
		t.Fatal("uniqSorted should de-duplicate")
	}

	if _, err := StateMigrate(true); err != nil {
		t.Fatalf("StateMigrate: %v", err)
	}
	if _, err := NormalizeStateKey("key"); err != nil {
		t.Fatalf("NormalizeStateKey: %v", err)
	}
	if _, err := StateWrite("facade-key", "content"); err != nil {
		t.Fatalf("StateWrite: %v", err)
	}
	read, err := StateRead("facade-key")
	if err != nil || read.Record.Content != "content" {
		t.Fatalf("StateRead = %#v, %v", read, err)
	}
	if _, err := StateList(); err != nil {
		t.Fatalf("StateList: %v", err)
	}
	if _, err := StateDoctor(); err != nil {
		t.Fatalf("StateDoctor: %v", err)
	}
	if _, err := StatePrune(time.Hour, false); err != nil {
		t.Fatalf("StatePrune: %v", err)
	}
	if StateDir() == "" {
		t.Fatal("StateDir should be non-empty")
	}

	if ok, _, err := ContextSerializationStable(func() any { return map[string]any{"b": 2, "a": 1} }); err != nil || !ok {
		t.Fatalf("ContextSerializationStable = %v, %v", ok, err)
	}
	if _, err := StableProjectionJSON(map[string]any{"x": 1}); err != nil {
		t.Fatalf("StableProjectionJSON: %v", err)
	}
	if StableProjection(map[string]any{"x": 1}) == nil {
		t.Fatal("StableProjection should return a value")
	}
	if len(ListDocs(repo)) == 0 || len(DocsIndex(repo, "v").Docs) == 0 {
		t.Fatal("docs wrappers should find README")
	}
	if title, headings := readDocHeadings(filepath.Join(repo, "README.md")); title != "Repo" || len(headings) == 0 {
		t.Fatalf("readDocHeadings = %q %#v", title, headings)
	}
	if !strings.Contains(shellQuote("a b"), "'") {
		t.Fatal("shellQuote should quote spaces")
	}
	if !strings.Contains(ExternalLLMPrintCommandPreview(), "zai") {
		t.Fatal("ExternalLLMPrintCommandPreview should mention zai")
	}
	var decoded map[string]any
	if err := DecodeExternalLLMStructuredJSONObject("test", []byte(`{"ok":true}`), &decoded); err != nil || decoded["ok"] != true {
		t.Fatalf("DecodeExternalLLMStructuredJSONObject = %#v %v", decoded, err)
	}
	if BuildExternalLLMJSONSchemaSection(`{"ok":true}`, []string{"ok boolean"}).Title == "" {
		t.Fatal("schema section should have title")
	}
	writeCoreTestFile(t, repo, ".env", "SECRET=value\n")
	if !GuardCheck(GuardCheckRequest{RepoRoot: repo, Files: []string{filepath.Join(repo, ".env")}}).OK {
		t.Fatal("GuardCheck wrapper should return a result")
	}
	if InspectHarness(repo, repo, t.TempDir(), "test", "skill").HarnessRoot == "" {
		t.Fatal("InspectHarness should return root info")
	}
	if exists(filepath.Join(repo, "README.md")) != true {
		t.Fatal("exists wrapper should detect file")
	}
	if _, err := ListSkillNames(repo); err == nil {
		t.Fatal("ListSkillNames on non-skill root should fail")
	}
	if DefaultNativeInstallRequest(repo, t.TempDir(), "", "", "bin").Root != repo {
		t.Fatal("DefaultNativeInstallRequest should preserve root")
	}
	if !strings.Contains(buildLintDiagnosePrompt(1, "panic"), "panic") {
		t.Fatal("buildLintDiagnosePrompt should include log")
	}
	if !strings.Contains(BuildStructuredPrompt(StructuredPromptSpec{Identity: "id", Objective: "obj"}), "obj") {
		t.Fatal("BuildStructuredPrompt should include objective")
	}
	if !strings.Contains(buildCommitSuggestPrompt("diff"), "diff") {
		t.Fatal("commit suggest prompt should include diff")
	}

	signals := AnalyzeProjectSignals(repo)
	if len(signals.Languages) == 0 || len(renderProjectDocs(repo, signals)) == 0 {
		t.Fatal("project docs signal/render wrappers should work")
	}
	if rendered := renderAgentsWithBlock(repo, ""); !strings.Contains(rendered, agentsStartMarker) {
		t.Fatal("renderAgentsWithBlock should include harness block")
	}
	if name, _, _, ok := parseDocFrontmatter(ensureDocMetaFrontmatter("TESTING.md", "# Testing")); !ok || name != "TESTING.md" {
		t.Fatalf("frontmatter parse failed: name=%q ok=%v", name, ok)
	}
	if len(ProjectDocNames()) == 0 || len(prefixedProjectDocNames()) == 0 {
		t.Fatal("project doc names should be non-empty")
	}
	if _, err := RouteProjectDocs(repo, "test"); err != nil {
		t.Fatalf("RouteProjectDocs: %v", err)
	}
	if _, err := ReadProjectDoc(repo, filepath.Join(ProjectDocsDir, "TESTING.md")); err != nil {
		t.Fatalf("ReadProjectDoc: %v", err)
	}
	if _, err := UpdateProjectDoc(ProjectDocsUpdateRequest{RepoRoot: repo, RelPath: filepath.Join(ProjectDocsDir, "ADR.md"), Content: "# ADR", Summary: "seed", Confirm: true}); err != nil {
		t.Fatalf("UpdateProjectDoc: %v", err)
	}
	if _, err := AppendProjectDocsRecord(ProjectDocsRecordRequest{RepoRoot: repo, Kind: "caution", Title: "t", Summary: "s"}); err != nil {
		t.Fatalf("AppendProjectDocsRecord: %v", err)
	}
}

func TestIssueOpsDraftWikiWorkflowAndWorkerFacades(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	repo := t.TempDir()
	writeCoreTestFile(t, repo, "go.mod", "module example.com/repo\n")

	record, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "1234-feature-test"})
	if err != nil {
		t.Fatalf("StartIssueOps: %v", err)
	}
	if _, err := ReadIssueOps(IssueOpsStateRoot(), record.ID); err != nil {
		t.Fatalf("ReadIssueOps: %v", err)
	}
	if newIssueOpsID(repo, "1234-feature-test") == "" || !IssueOpsPhaseExpectsWorktree(IssueOpsPhaseImplement) {
		t.Fatal("issueops phase helpers should work")
	}
	record.Intent = &IssueOpsIntentContract{RawRequest: "raw", InterpretedIntent: "intent", SuccessCriteria: []string{"done"}}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatalf("writeIssueOps: %v", err)
	}
	if _, err := LinkIssueOpsIssue(IssueOpsStateRoot(), record.ID, "https://github.com/acme/repo/issues/1"); err != nil {
		t.Fatalf("LinkIssueOpsIssue: %v", err)
	}
	if _, err := LinkIssueOpsPlan(IssueOpsStateRoot(), record.ID, filepath.Join(repo, "PLAN.md")); err == nil {
		t.Fatal("LinkIssueOpsPlan should fail for missing plan file")
	}
	if _, err := LinkIssueOpsChild(IssueOpsStateRoot(), record.ID, "https://github.com/acme/repo/issues/2", "child"); err != nil {
		t.Fatalf("LinkIssueOpsChild: %v", err)
	}
	if _, err := LinkIssueOpsRelated(IssueOpsStateRoot(), record.ID, "follows-up", "https://github.com/acme/repo/issues/3", "related"); err != nil {
		t.Fatalf("LinkIssueOpsRelated: %v", err)
	}
	if validateIssueOpsIssueBranch("1234-feature-test") != nil {
		t.Fatal("valid issue branch rejected")
	}
	if _, err := AddIssueOpsFeedback(IssueOpsStateRoot(), record.ID, "review", "body", "defect"); err != nil {
		t.Fatalf("AddIssueOpsFeedback: %v", err)
	}
	if _, err := AddIssueOpsDecision(IssueOpsStateRoot(), record.ID, IssueOpsDecisionRecordRequest{Title: "decision", Body: "body", Kind: "implementation"}); err != nil {
		t.Fatalf("AddIssueOpsDecision: %v", err)
	}
	record, _ = ReadIssueOps(IssueOpsStateRoot(), record.ID)
	if IssueOpsPRReadiness(record).Ready || IssueOpsStrictPRReadiness(record).Ready {
		t.Fatal("incomplete record should not be PR-ready")
	}
	_ = IssueOpsImplementationReadiness(record)
	_ = IssueOpsPlanReadiness(record)
	_ = IssueOpsAISlopCleanReadiness(record)
	_ = IssueOpsCleanupStatusForRecord(record, IssueOpsCleanupStatusRequest{})
	_, _ = IssueOpsCleanupStatusByID(IssueOpsStateRoot(), record.ID, IssueOpsCleanupStatusRequest{})
	if ScanStaleIssueOpsCycles(IssueOpsStaleScanRequest{Repo: repo}).Repo == "" {
		t.Fatal("ScanStaleIssueOpsCycles should return repo")
	}
	if _, err := resolveProvider("bad"); err == nil {
		t.Fatal("unknown provider should fail")
	}
	if _, err := DecodeIssueOpsBenchmarkJudgeJSON([]byte(`{"ok":true,"fixture_id":"f1","average_score":90,"minimum_score":80,"dimension_scores":[{"dimension":"intent_understanding","score":90,"evidence":"ok"}],"passed":true}`)); err != nil {
		t.Fatalf("DecodeIssueOpsBenchmarkJudgeJSON: %v", err)
	}
	if _, err := DecodeIssueOpsRemoteScoringRequest([]byte(`{"provider":"github","issue":{"title":"Bug","body":"Fix"},"issue_candidates":[]}`)); err != nil {
		t.Fatalf("DecodeIssueOpsRemoteScoringRequest: %v", err)
	}
	if _, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{Issue: IssueOpsRemoteArtifact{Title: "Bug fix"}}); err != nil {
		t.Fatalf("ScoreIssueOpsRemoteCandidates: %v", err)
	}
	if ExpectedWorktreeFromSession(repo, func() string { return "fallback" }) == "" {
		t.Fatal("ExpectedWorktreeFromSession should return fallback")
	}
	_ = IssueOpsResume(repo)
	_ = IssueOpsLastActiveAt(record)

	initResult, err := InitDraftWiki(DraftWikiInitRequest{RepoRoot: repo, Write: true})
	if err != nil || !initResult.OK {
		t.Fatalf("InitDraftWiki = %#v %v", initResult, err)
	}
	if _, err := ListDraftWiki(DraftWikiListRequest{RepoRoot: repo}); err != nil {
		t.Fatalf("ListDraftWiki: %v", err)
	}
	if len(draftWikiSeedFiles()) == 0 || draftWikiRawFileName("2026-06-13", "Draft.md") == "" {
		t.Fatal("draft wiki helpers should return data")
	}
	if !strings.Contains(generatedDraftFrontmatter("Title", "wiki", "notes", "agy"), "Title") {
		t.Fatal("generated frontmatter should include title")
	}
	if !strings.Contains(buildDraftWikiSuggestPrompt(DraftWikiSuggestRequest{RepoRoot: repo}, "input", "agy", "notes"), "input") {
		t.Fatal("suggest prompt should include input")
	}
	event := DraftWikiQueueEvent{ID: "id", Kind: "draft", SourceMaterial: "material", Status: "pending"}
	if failDraftWikiQueueEvent(event, errors.New("boom")).Status == "" {
		t.Fatal("fail queue event should set status")
	}
	if trimDraftWikiQueueMaterial("  a  ") != "a" || draftWikiQueueEventID("repo", "material", "now") == "" {
		t.Fatal("queue helpers should normalize material/id")
	}
	_, queuePath, err := draftWikiQueuePath(repo, true)
	if err != nil || queuePath == "" {
		t.Fatalf("draftWikiQueuePath = %q %v", queuePath, err)
	}
	if err := appendDraftWikiQueueEvent(queuePath, event); err != nil {
		t.Fatalf("appendDraftWikiQueueEvent: %v", err)
	}
	if count, err := countDraftWikiQueueLines(queuePath, 10); err != nil || count != 1 {
		t.Fatalf("countDraftWikiQueueLines = %d %v", count, err)
	}
	if events, _, err := readDraftWikiQueueEvents(queuePath); err != nil || len(events) != 1 {
		t.Fatalf("readDraftWikiQueueEvents = %#v %v", events, err)
	}
	if !strings.Contains(formatDraftWikiQueueMalformedWarning(1, "{"), "malformed") {
		t.Fatal("format malformed warning should describe malformed line")
	}
	if err := capDraftWikiQueueEvents(queuePath, 1); err != nil {
		t.Fatalf("capDraftWikiQueueEvents: %v", err)
	}
	if _, err := pruneDraftWikiQueuePath(queuePath, 1); err != nil {
		t.Fatalf("pruneDraftWikiQueuePath: %v", err)
	}
	if err := rewriteDraftWikiQueueEvents(queuePath, []DraftWikiQueueEvent{event}); err != nil {
		t.Fatalf("rewriteDraftWikiQueueEvents: %v", err)
	}
	unlock, acquired, err := acquireDraftWikiQueueLock(filepath.Dir(queuePath))
	if err != nil || !acquired {
		t.Fatalf("acquireDraftWikiQueueLock acquired=%v err=%v", acquired, err)
	}
	unlock()

	job, err := EnqueueWorkerJob("read_only", "{}")
	if err != nil || job.ID == "" {
		t.Fatalf("EnqueueWorkerJob = %#v %v", job, err)
	}
	if _, err := ReadWorkerJob(job.ID); err != nil {
		t.Fatalf("ReadWorkerJob: %v", err)
	}
	if _, err := ListWorkerJobs(); err != nil {
		t.Fatalf("ListWorkerJobs: %v", err)
	}
	if _, err := CancelWorkerJob(job.ID); err != nil {
		t.Fatalf("CancelWorkerJob: %v", err)
	}
	if _, err := RunReadOnlyWorkerJob("read_only", "{}", CommandPolicyRequest{WorkspaceRoot: repo, CWD: repo, Argv: []string{"git", "status"}}); err != nil {
		t.Fatalf("RunReadOnlyWorkerJob: %v", err)
	}
}

func TestHookAndLifecycleFacades(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	repo := t.TempDir()
	result := BuildUserPromptMCPHints(HookUserPromptRequest{Repo: repo, Prompt: "commit and test"})
	if len(result.Hints) == 0 || result.AdditionalContext == "" {
		t.Fatalf("unexpected hook prompt result: %#v", result)
	}
	if renderHookMCPHintContext(result.Hints, nil, nil, "catalog") == "" {
		t.Fatal("renderHookMCPHintContext should render context")
	}
	var parts []string
	appendCompactPendingUpkeep(&parts, []DocUpkeepEvent{{Kind: "test", TargetDocs: []string{"TESTING.md"}, Summary: "update", Status: "pending"}})
	if fallbackHintPriority(result.Hints[0]) == "" || compactHintLabel(result.Hints[0]) == "" {
		t.Fatal("hint helpers should return labels")
	}
	if !containsAnySlice("abcdef", []string{"bc"}) || !containsAny("abcdef", "de") {
		t.Fatal("contains helpers should match")
	}
	if BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{Repo: repo}).OK == false {
		t.Fatal("pre tool use decision should return result")
	}
	_ = BuildLifecycleStopReminder(repo)
	_ = BuildLifecyclePreCompactCapsule(repo)
	_ = BuildLifecyclePostCompactReminder(repo)
	if _, err := InitProjectLifecycleState(repo, true, ProjectProfile{Languages: []string{"Go"}}); err != nil {
		t.Fatalf("InitProjectLifecycleState: %v", err)
	}
	if _, err := ResolveProjectLifecycleState(repo); err != nil {
		t.Fatalf("ResolveProjectLifecycleState: %v", err)
	}
	if _, err := ValidateProjectLifecycleState(repo); err != nil {
		t.Fatalf("ValidateProjectLifecycleState: %v", err)
	}
	if _, err := AppendDocUpkeepEvent(repo, DocUpkeepEvent{Kind: "test", TargetDocs: []string{"TESTING.md"}, Summary: "update", Status: "pending"}); err != nil {
		t.Fatalf("AppendDocUpkeepEvent: %v", err)
	}
	if events, _, err := ReadPendingDocUpkeepEvents(repo, 10); err != nil || len(events) == 0 {
		t.Fatalf("ReadPendingDocUpkeepEvents = %#v %v", events, err)
	}
	trigger := BuildNextActionJudgementTrigger("선택지:\n1. go (추천)\n2. stop\n3. wait")
	if !trigger.ShouldReenterAgent || BuildNextActionJudgementRelayReason(trigger) == "" {
		t.Fatalf("unexpected trigger: %#v", trigger)
	}
	if !BuildNumberedNextActionsDecision("done", true, "test").OK {
		t.Fatal("BuildNumberedNextActionsDecision should return OK result")
	}
	if !IsNoAutoProceedJudgement("no-auto-proceed") {
		t.Fatal("no-auto-proceed should be detected")
	}
	if EvaluateNextActionAutoProceed("선택지:\n1. go (추천)\n2. stop\n3. wait", 0.1).SelectedText == "" {
		t.Fatal("auto proceed should parse recommended candidate")
	}
	candidates := parseNextActionCandidates("선택지:\n1. go (추천)\n2. stop\n3. wait")
	if len(candidates) != 3 || selectRecommendedNextAction(candidates) == nil || buildNextActionAutoProceedLLMPrompt(candidates[0], candidates) == "" {
		t.Fatalf("unexpected next action candidates: %#v", candidates)
	}
	if RecordStopNextActionRelay(repo, trigger).OK == false || ClearStopNextActionRelay(repo).OK == false {
		t.Fatal("relay record/clear should succeed")
	}
}

func writeCoreTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
