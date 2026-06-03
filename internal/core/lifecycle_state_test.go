package core

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveProjectLifecycleNamespaceIsProjectScoped(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateRoot)
	repoA := t.TempDir()
	repoB := t.TempDir()
	mustWrite(t, filepath.Join(repoA, ".git", "config"), "[remote \"origin\"]\n\turl = git@example.com:a/repo.git\n")
	mustWrite(t, filepath.Join(repoB, ".git", "config"), "[remote \"origin\"]\n\turl = git@example.com:b/repo.git\n")

	a, err := ResolveProjectLifecycleState(repoA)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ResolveProjectLifecycleState(repoB)
	if err != nil {
		t.Fatal(err)
	}
	if a.RepoID == "" || b.RepoID == "" || a.RepoID == b.RepoID {
		t.Fatalf("repo ids should be non-empty and distinct: a=%q b=%q", a.RepoID, b.RepoID)
	}
	if !strings.HasPrefix(a.ProjectStateDir, filepath.Join(stateRoot, "projects")) {
		t.Fatalf("state dir not under project namespace: %s", a.ProjectStateDir)
	}
	if a.ProjectStateDir == b.ProjectStateDir {
		t.Fatalf("project state dirs should differ: %s", a.ProjectStateDir)
	}
}

func TestInitProjectLifecycleStateWritesProjectJSONOnlyWhenConfirmed(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()

	dry, err := InitProjectLifecycleState(repo, false)
	if err != nil {
		t.Fatal(err)
	}
	if !dry.OK || dry.Exists || dry.NamespaceValid || dry.ProjectJSONPath == "" {
		t.Fatalf("unexpected dry lifecycle state: %+v", dry)
	}
	if _, err := os.Stat(dry.ProjectJSONPath); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote project.json or unexpected stat error: %v", err)
	}

	written, err := InitProjectLifecycleState(repo, true)
	if err != nil {
		t.Fatal(err)
	}
	if !written.OK || !written.Exists || !written.NamespaceValid {
		t.Fatalf("unexpected written lifecycle state: %+v", written)
	}
	if _, err := os.Stat(written.ProjectJSONPath); err != nil {
		t.Fatalf("project.json missing: %v", err)
	}
}

func TestValidateProjectLifecycleStateDetectsNamespaceMismatch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	written, err := InitProjectLifecycleState(repo, true)
	if err != nil {
		t.Fatal(err)
	}
	var profile ProjectLifecycleProfile
	b, err := os.ReadFile(written.ProjectJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &profile); err != nil {
		t.Fatal(err)
	}
	profile.Fingerprint.RepoRoot = filepath.Join(t.TempDir(), "other")
	b, err = json.MarshalIndent(profile, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(written.ProjectJSONPath, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	validated, err := ValidateProjectLifecycleState(repo)
	if err != nil {
		t.Fatal(err)
	}
	if validated.NamespaceValid || !containsString(validated.Warnings, "namespace_mismatch") {
		t.Fatalf("expected namespace mismatch warning: %+v", validated)
	}
}

func TestAppendDocUpkeepEventWritesJSONL(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := InitProjectLifecycleState(repo, true); err != nil {
		t.Fatal(err)
	}
	result, err := AppendDocUpkeepEvent(repo, DocUpkeepEvent{
		Kind:       "operation_change",
		TargetDocs: []string{"OPERATIONS.md"},
		Summary:    "Hook behavior changed.",
		Evidence:   []string{"internal/core/hook_prompt.go"},
		Source:     "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Path == "" || result.Event.ID == "" {
		t.Fatalf("unexpected append result: %+v", result)
	}
	file, err := os.Open(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("expected one jsonl record")
	}
	var got DocUpkeepEvent
	if err := json.Unmarshal(scanner.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Kind != "operation_change" || got.TargetDocs[0] != "OPERATIONS.md" || got.Status != "pending" {
		t.Fatalf("unexpected event: %+v", got)
	}
	if scanner.Scan() {
		t.Fatalf("expected one event, got extra line: %s", scanner.Text())
	}
}

func TestPreToolUseSearchRoutingBlocksRawStructuralSourceSearch(t *testing.T) {
	for _, command := range []string{
		`/usr/bin/rg -n "func Run" cmd/internal`,
		`rg "func Run"`,
		`rg "func Run" .`,
		`git grep "func Run"`,
		`rg "func Run" docs/ cmd/`,
		`rg "func Run" controllers/`,
		`rg "func Run" cmd/ # codegraph`,
	} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Tool:                 "zsh",
			Command:              command,
			EnforceSearchRouting: true,
		})
		if got.Decision != "block" {
			t.Fatalf("expected command to be blocked: %q -> %+v", command, got)
		}
	}
}

func TestPreToolUseSearchRoutingAllowsRawExactLiteralSearch(t *testing.T) {
	for _, command := range []string{
		`rg "DATABASE_URL" internal config`,
		`rg "PostToolUseFailure" internal/core`,
		`rg "Cannot read property" src`,
		`rg "snapshot_manager.go" internal`,
		`rg "pattern" snapshot_manager.go`,
		`rg "pattern" ./snapshot_manager.go`,
	} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Tool:                 "zsh",
			Command:              command,
			EnforceSearchRouting: true,
		})
		if got.Decision != "allow" {
			t.Fatalf("expected exact literal search to be allowed: %q -> %+v", command, got)
		}
	}
}

func TestPreToolUseSearchRoutingBlocksCodexShellToolNames(t *testing.T) {
	for _, tool := range []string{"shell_command", "unified_exec", "exec_command"} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Tool:                 tool,
			Command:              `rg "type Hook" internal/core`,
			EnforceSearchRouting: true,
		})
		if got.Decision != "block" {
			t.Fatalf("expected Codex shell tool %q to be blocked, got %+v", tool, got)
		}
	}
}

func TestPreToolUseSearchRoutingAllowsDocsLiteralCodeNames(t *testing.T) {
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Tool:                 "Bash",
		Command:              `rg "main.go" docs/ README.md`,
		EnforceSearchRouting: true,
	})
	if got.Decision != "allow" {
		t.Fatalf("expected docs literal search to be allowed: %+v", got)
	}
}

func TestPreToolUseSearchRoutingAllowsExternalAbsoluteTargets(t *testing.T) {
	repo := t.TempDir()
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:                 repo,
		Tool:                 "Bash",
		Command:              `grep -R "PostToolUse" -n /Applications/Codex.app/Contents/Resources`,
		EnforceSearchRouting: true,
	})
	if got.Decision != "allow" {
		t.Fatalf("expected external absolute target search to be allowed: %+v", got)
	}
}

func TestPreToolUseSearchRoutingBlocksCodeGraphForExactSearch(t *testing.T) {
	for _, query := range []string{"DATABASE_URL", "PostToolUseFailure", "Cannot read property", "snapshot_manager.go", "TODO"} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Tool:                 "codegraph_context",
			Command:              query,
			EnforceSearchRouting: true,
		})
		if got.Decision != "block" {
			t.Fatalf("expected exact CodeGraph query to be blocked: %q -> %+v", query, got)
		}
	}
}

func TestPreToolUseSearchRoutingAllowsCodeGraphForStructuralSearch(t *testing.T) {
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Tool:                 "codegraph_trace",
		Command:              "impact of changing BuildLifecyclePreToolUseDecision",
		EnforceSearchRouting: true,
	})
	if got.Decision != "allow" {
		t.Fatalf("expected structural CodeGraph query to be allowed: %+v", got)
	}
}

func TestPreToolUseWorktreeGuardBlocksSourceCheckoutMutation(t *testing.T) {
	source := filepath.Join(t.TempDir(), "agent-harness")
	worktree := filepath.Join(t.TempDir(), "agent-harness.worktrees", "chore-19-docs")
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:             source,
		Tool:             "apply_patch",
		Paths:            []string{filepath.Join(source, ".agent-harness", "OPERATIONS.md")},
		EnforceWorktree:  true,
		ExpectedWorktree: worktree,
		SourceCheckout:   source,
	})
	if got.Decision != "block" || !strings.Contains(got.Reason, "expected IssueOps worktree") {
		t.Fatalf("expected source checkout mutation to be blocked: %+v", got)
	}
}

func TestPreToolUseWorktreeGuardAllowsExpectedWorktreeMutation(t *testing.T) {
	source := filepath.Join(t.TempDir(), "agent-harness")
	worktree := filepath.Join(t.TempDir(), "agent-harness.worktrees", "chore-19-docs")
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:             worktree,
		Tool:             "apply_patch",
		Paths:            []string{filepath.Join(worktree, ".agent-harness", "OPERATIONS.md")},
		EnforceWorktree:  true,
		ExpectedWorktree: worktree,
		SourceCheckout:   source,
	})
	if got.Decision != "allow" {
		t.Fatalf("expected worktree mutation to be allowed: %+v", got)
	}
}

func TestPreToolUseWorktreeGuardNoopsWithoutExpectedWorktree(t *testing.T) {
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:            t.TempDir(),
		Tool:            "apply_patch",
		Paths:           []string{".agent-harness/OPERATIONS.md"},
		EnforceWorktree: true,
	})
	if got.Decision != "allow" {
		t.Fatalf("missing expected worktree should not block: %+v", got)
	}
}

func TestPreToolUseKoreanRemoteArtifactGateBlocksEnglishPR(t *testing.T) {
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:                t.TempDir(),
		Tool:                "bash",
		Command:             `gh pr create --title "Document split and IssueOps guardrails" --body "Summary Changes Verification Risk"`,
		EnforceKoreanRemote: true,
	})
	if got.Decision != "block" || !strings.Contains(got.Reason, "IssueOps remote artifact gate failed") {
		t.Fatalf("expected English PR artifact to be blocked: %+v", got)
	}
}

func TestPreToolUseKoreanRemoteArtifactGateAllowsKoreanPRBodyFile(t *testing.T) {
	repo := t.TempDir()
	body := "## 요약\n\n- 문서 분할과 hook guard를 추가했습니다.\n- 검증 명령과 위험도를 한국어로 기록했습니다.\n"
	if err := os.WriteFile(filepath.Join(repo, "pr-body.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:                repo,
		Tool:                "bash",
		Command:             `gh pr create --title "문서 분할과 IssueOps guardrail 추가" --body-file pr-body.md`,
		EnforceKoreanRemote: true,
	})
	if got.Decision != "allow" {
		t.Fatalf("expected Korean PR artifact to be allowed: %+v", got)
	}
}

func TestNumberedNextActionsDecisionBlocksMissingChoices(t *testing.T) {
	got := BuildNumberedNextActionsDecision("작업했습니다.", true, "stop")
	if got.Decision != "block" || !strings.Contains(got.Reason, "numbered next actions") {
		t.Fatalf("expected missing numbered next actions to block, got %+v", got)
	}
}

func TestNumberedNextActionsDecisionAllowsChoices(t *testing.T) {
	got := BuildNumberedNextActionsDecision(`완료했습니다.

선택지:
1. 진행: 다음 검증을 실행합니다. (추천)
2. 축소 진행: 작은 범위만 확인합니다.
3. 보류: 여기서 멈춥니다.`, true, "stop")
	if got.Decision != "allow" {
		t.Fatalf("expected numbered choices to allow, got %+v", got)
	}
}

func TestNumberedNextActionsDecisionAllowsMarkdownListChoices(t *testing.T) {
	got := BuildNumberedNextActionsDecision(`완료했습니다.

선택지:
- 1. 진행: 다음 검증을 실행합니다. (추천)
* 2. 축소 진행: 작은 범위만 확인합니다.
+ 3. 보류: 여기서 멈춥니다.`, true, "stop")
	if got.Decision != "allow" {
		t.Fatalf("expected markdown list numbered choices to allow, got %+v", got)
	}
}

func TestNumberedNextActionsDecisionNoopsWhenDisabled(t *testing.T) {
	got := BuildNumberedNextActionsDecision("작업했습니다.", false, "stop")
	if got.Decision != "allow" {
		t.Fatalf("expected disabled guard to allow, got %+v", got)
	}
}

func TestRecordLifecycleToolUseQueuesRelevantDocUpkeep(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := InitProjectLifecycleState(repo, true); err != nil {
		t.Fatal(err)
	}
	result, err := RecordLifecycleToolUse(HookToolUseLifecycleRequest{
		Repo:   repo,
		Tool:   "apply_patch",
		Paths:  []string{"internal/core/hook_prompt.go", "internal/core/hook_prompt_test.go"},
		Source: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.Recorded || result.Event.ID == "" {
		t.Fatalf("expected recorded upkeep event: %+v", result)
	}
	if !containsString(result.Event.TargetDocs, "OPERATIONS.md") || !containsString(result.Event.TargetDocs, "TESTING.md") {
		t.Fatalf("expected operations/testing targets: %+v", result.Event.TargetDocs)
	}
}

func TestBuildLifecycleStopReminderIncludesPendingUpkeep(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := InitProjectLifecycleState(repo, true); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendDocUpkeepEvent(repo, DocUpkeepEvent{Kind: "code_change", TargetDocs: []string{"OPERATIONS.md"}, Summary: "Hook behavior changed.", Source: "test"}); err != nil {
		t.Fatal(err)
	}
	reminder := BuildLifecycleStopReminder(repo)
	if !reminder.ShouldInject || !strings.Contains(reminder.AdditionalContext, "Pending .agent-harness doc upkeep") || !strings.Contains(reminder.AdditionalContext, "OPERATIONS.md") {
		t.Fatalf("unexpected reminder: %+v", reminder)
	}
}

func TestLifecycleCompactCapsulePreservesPendingUpkeep(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := InitProjectLifecycleState(repo, true); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendDocUpkeepEvent(repo, DocUpkeepEvent{
		Kind:       "code_change",
		TargetDocs: []string{"OPERATIONS.md", "TESTING.md"},
		Summary:    "Hook and tests changed.",
		Source:     "test",
	}); err != nil {
		t.Fatal(err)
	}

	pre := BuildLifecyclePreCompactCapsule(repo)
	if !pre.OK || !pre.Recorded || pre.PendingCount != 1 || pre.CompactPath == "" {
		t.Fatalf("unexpected pre-compact result: %+v", pre)
	}
	if _, err := os.Stat(pre.CompactPath); err != nil {
		t.Fatalf("compact capsule missing: %v", err)
	}

	post := BuildLifecyclePostCompactReminder(repo)
	if !post.OK || !post.ShouldInject || post.PendingCount != 1 {
		t.Fatalf("unexpected post-compact result: %+v", post)
	}
	for _, want := range []string{"Restored agent-harness compaction capsule", "OPERATIONS.md", "TESTING.md", "UserPromptSubmit will keep surfacing the current details"} {
		if !strings.Contains(post.AdditionalContext, want) {
			t.Fatalf("post-compact context missing %q: %s", want, post.AdditionalContext)
		}
	}
	if strings.Contains(post.AdditionalContext, "Hook and tests changed.") {
		t.Fatalf("post-compact should not duplicate detailed pending-upkeep summaries:\n%s", post.AdditionalContext)
	}
	if _, err := os.Stat(pre.CompactPath); !os.IsNotExist(err) {
		t.Fatalf("post-compact should consume compact capsule, stat error: %v", err)
	}
}

func TestLifecyclePreCompactNoPendingUpkeepDoesNotWriteCapsule(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := InitProjectLifecycleState(repo, true); err != nil {
		t.Fatal(err)
	}
	pre := BuildLifecyclePreCompactCapsule(repo)
	if !pre.OK || pre.Recorded || pre.PendingCount != 0 {
		t.Fatalf("unexpected pre-compact result: %+v", pre)
	}
	if _, err := os.Stat(pre.CompactPath); !os.IsNotExist(err) {
		t.Fatalf("pre-compact without pending upkeep should not write capsule, stat error: %v", err)
	}
}

func TestEvaluateNextActionAutoProceedAdvancesRecommendedSafeAction(t *testing.T) {
	message := strings.Join([]string{
		"구현을 마쳤습니다.",
		"선택지:",
		"1. 진행: 다음 단계 테스트를 추가하고 구현을 계속합니다. (추천)",
		"2. 축소 진행: 일부만 먼저 검증합니다.",
		"3. 보류: 현재 상태로 멈추고 사용자 확인을 기다립니다.",
	}, "\n")
	result := EvaluateNextActionAutoProceed(message, 0)
	if !result.OK {
		t.Fatalf("expected ok result, got %+v", result)
	}
	if !result.AutoProceed {
		t.Fatalf("recommended safe reversible action should auto-proceed, got %+v", result)
	}
	if result.SelectedIndex != 1 {
		t.Fatalf("expected selected index 1, got %d", result.SelectedIndex)
	}
	if result.TopScore < result.Threshold {
		t.Fatalf("top score %.2f should meet threshold %.2f", result.TopScore, result.Threshold)
	}
}

func TestEvaluateNextActionAutoProceedNeverAdvancesDestructiveCleanup(t *testing.T) {
	message := strings.Join([]string{
		"머지 상태를 확인했습니다.",
		"선택지:",
		"1. 정리 진행: merged PR worktree와 local branch를 삭제합니다. (추천)",
		"2. 보류: worktree는 유지합니다.",
		"3. 확장 정리: 전체 stale worktree를 점검합니다.",
	}, "\n")
	result := EvaluateNextActionAutoProceed(message, 0)
	if result.AutoProceed {
		t.Fatalf("destructive recommended action must not auto-proceed, got %+v", result)
	}
	if result.BlockedByGuard == "" {
		t.Fatalf("expected a guard reason for destructive action, got %+v", result)
	}
}

func TestEvaluateNextActionAutoProceedStopsOnAmbiguousChoices(t *testing.T) {
	message := strings.Join([]string{
		"해석이 갈립니다.",
		"선택지:",
		"1. 해석 A로 구현합니다.",
		"2. 해석 B로 구현합니다.",
		"3. 해석 C로 구현합니다.",
	}, "\n")
	result := EvaluateNextActionAutoProceed(message, 0)
	if result.AutoProceed {
		t.Fatalf("no explicit recommendation should not auto-proceed, got %+v", result)
	}
}

func TestEvaluateNextActionAutoProceedNoChoicesDoesNotProceed(t *testing.T) {
	result := EvaluateNextActionAutoProceed("작업을 완료했습니다.", 0)
	if result.AutoProceed {
		t.Fatalf("message without numbered choices must not auto-proceed, got %+v", result)
	}
}

func TestEvaluateNextActionAutoProceedRespectsHighThreshold(t *testing.T) {
	message := strings.Join([]string{
		"선택지:",
		"1. 진행: 구현을 계속합니다. (추천)",
		"2. 축소 진행: 일부만 검증합니다.",
		"3. 보류: 멈춥니다.",
	}, "\n")
	result := EvaluateNextActionAutoProceed(message, 1.01)
	if result.AutoProceed {
		t.Fatalf("threshold above max must block auto-proceed, got %+v", result)
	}
}
