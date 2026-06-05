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

func TestPreToolUseWorktreeGuardRequiresCodeGraphProjectPath(t *testing.T) {
	source := filepath.Join(t.TempDir(), "agent-harness")
	worktree := filepath.Join(t.TempDir(), "agent-harness.worktrees", "chore-19-docs")
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:             source,
		Tool:             "mcp__codegraph__codegraph_search",
		Command:          "BuildLifecyclePreToolUseDecision",
		EnforceWorktree:  true,
		ExpectedWorktree: worktree,
		SourceCheckout:   source,
	})
	if got.Decision != "block" || !strings.Contains(got.Reason, "projectPath") {
		t.Fatalf("expected CodeGraph without projectPath to be blocked: %+v", got)
	}
}

func TestPreToolUseWorktreeGuardAllowsCodeGraphExpectedProjectPath(t *testing.T) {
	source := filepath.Join(t.TempDir(), "agent-harness")
	worktree := filepath.Join(t.TempDir(), "agent-harness.worktrees", "chore-19-docs")
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:             source,
		Tool:             "mcp__codegraph__codegraph_search",
		Command:          "BuildLifecyclePreToolUseDecision",
		EnforceWorktree:  true,
		ExpectedWorktree: worktree,
		SourceCheckout:   source,
		ProjectPath:      worktree,
	})
	if got.Decision != "allow" {
		t.Fatalf("expected CodeGraph with worktree projectPath to be allowed: %+v", got)
	}
}

func TestPreToolUseWorktreeGuardBlocksSourceBoundMCPTools(t *testing.T) {
	source := filepath.Join(t.TempDir(), "agent-harness")
	worktree := filepath.Join(t.TempDir(), "agent-harness.worktrees", "chore-19-docs")
	for _, tool := range []string{"mcp__filesystem__read_file", "mcp__plugin_serena_serena__find_symbol"} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Repo:             source,
			Tool:             tool,
			Command:          "BuildLifecyclePreToolUseDecision",
			EnforceWorktree:  true,
			ExpectedWorktree: worktree,
			SourceCheckout:   source,
		})
		if got.Decision != "block" || !strings.Contains(got.Reason, "IssueOps worktree") {
			t.Fatalf("expected %s to be blocked in IssueOps worktree context: %+v", tool, got)
		}
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

func TestPreToolUseRemoteArtifactGateAllowsHelpCommands(t *testing.T) {
	for _, command := range []string{
		`gh issue create --help`,
		`gh pr create -h`,
		`glab issue create --help`,
		`glab mr create -h`,
	} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Repo:                t.TempDir(),
			Tool:                "bash",
			Command:             command,
			EnforceKoreanRemote: true,
			EnforceVCSLinking:   true,
		})
		if got.Decision != "allow" {
			t.Fatalf("expected help command to be allowed: %q -> %+v", command, got)
		}
	}
}

func TestPreToolUseVCSLinkingBlocksRemoteCreateWithoutLabels(t *testing.T) {
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:              t.TempDir(),
		Tool:              "bash",
		Command:           `glab mr create --title "IssueOps 라벨 검증" --description "라벨 없는 MR 생성을 막고 이슈 라벨 복사 또는 수동 라벨 적용을 강제합니다."`,
		EnforceVCSLinking: true,
	})
	if got.Decision != "block" || !strings.Contains(got.Reason, "label") {
		t.Fatalf("expected unlabeled remote create to be blocked: %+v", got)
	}
}

func TestPreToolUseVCSLinkingBlocksRemoteCreateWithoutAssignee(t *testing.T) {
	for _, command := range []string{
		`glab mr create --title "IssueOps 담당자 검증" --description "라벨은 있지만 담당자 없는 MR 생성을 막습니다." --label bug`,
		`glab mr create --title "IssueOps 담당자 검증" --description "이슈 라벨 복사 옵션이 있어도 담당자는 필요합니다." --related-issue 2385 --copy-issue-labels`,
	} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Repo:              t.TempDir(),
			Tool:              "bash",
			Command:           command,
			EnforceVCSLinking: true,
		})
		if got.Decision != "block" || !strings.Contains(got.Reason, "assignee") {
			t.Fatalf("expected unassigned remote create to be blocked: %q -> %+v", command, got)
		}
	}
}

func TestPreToolUseVCSLinkingAllowsRemoteCreateWithLabelsAndAssignee(t *testing.T) {
	for _, command := range []string{
		`glab mr create --title "IssueOps 라벨 검증" --description "이슈 라벨을 복사해 MR 라벨 누락을 방지합니다." --label bug --assignee m16khb`,
		`glab mr create --title "IssueOps 라벨 검증" --description "이슈 라벨 복사와 담당자를 함께 지정합니다." --copy-issue-labels --assignee-id 100`,
		`gh pr create --title "IssueOps 라벨 검증" --body "라벨과 담당자를 함께 지정합니다." -l bug -a @me`,
	} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Repo:              t.TempDir(),
			Tool:              "bash",
			Command:           command,
			EnforceVCSLinking: true,
		})
		if got.Decision != "allow" {
			t.Fatalf("expected labeled and assigned remote create to be allowed: %q -> %+v", command, got)
		}
	}
}

func TestPreToolUseGitOpsKubectlBlocksMutatingCommands(t *testing.T) {
	for _, command := range []string{
		`kubectl apply -f k8s/deployment.yaml`,
		`/usr/local/bin/kubectl delete pod api-0 -n prod`,
		`kubectl get pods && kubectl rollout restart deployment/api -n prod`,
	} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Tool:                 "bash",
			Command:              command,
			EnforceGitOpsKubectl: true,
		})
		if got.Decision != "block" || !strings.Contains(got.Reason, "GitOps") {
			t.Fatalf("expected mutating kubectl command to be blocked: %q -> %+v", command, got)
		}
	}
}

func TestPreToolUseGitOpsKubectlAsksForLiveAccessCommands(t *testing.T) {
	for _, command := range []string{
		`kubectl exec -it pod/api-0 -- sh`,
		`kubectl port-forward svc/api 8080:80 -n prod`,
		`kubectl get pods && kubectl exec deployment/api -- env`,
	} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Tool:                 "bash",
			Command:              command,
			EnforceGitOpsKubectl: true,
		})
		if got.Decision != "ask" || !strings.Contains(got.Reason, "kubectl") || !strings.Contains(got.Reason, "confirm") {
			t.Fatalf("expected live kubectl access to ask for confirmation: %q -> %+v", command, got)
		}
	}
}

func TestPreToolUseGitOpsKubectlAllowsReadOnlyCommands(t *testing.T) {
	for _, command := range []string{
		`kubectl get pods -n prod`,
		`kubectl logs deployment/api -n prod --tail=100`,
		`kubectl diff -f k8s/`,
		`kubectl apply --dry-run=client -f k8s/deployment.yaml`,
		`kubectl apply --dry-run=server -f k8s/deployment.yaml`,
		`kubectl apply --dry-run server -f k8s/deployment.yaml`,
	} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Tool:                 "bash",
			Command:              command,
			EnforceGitOpsKubectl: true,
		})
		if got.Decision != "allow" {
			t.Fatalf("expected read-only kubectl command to be allowed: %q -> %+v", command, got)
		}
	}
}

func TestPreToolUseStagedChecksAsksForBroadBiomeCommands(t *testing.T) {
	for _, command := range []string{
		`npx biome check apps libs`,
		`biome format --check apps libs`,
		`npm run lint:check`,
	} {
		repo := t.TempDir()
		if err := os.WriteFile(filepath.Join(repo, "package.json"), []byte(`{"scripts":{"lint:check":"biome check apps libs"}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Repo:                repo,
			Tool:                "bash",
			Command:             command,
			EnforceStagedChecks: true,
		})
		if got.Decision != "ask" || !strings.Contains(got.Reason, "staged") {
			t.Fatalf("expected broad staged check to ask for confirmation: %q -> %+v", command, got)
		}
	}
}

func TestPreToolUseStagedChecksAllowsScopedBiomeCommands(t *testing.T) {
	for _, command := range []string{
		`npx biome check --staged --no-errors-on-unmatched`,
		`biome format --staged`,
		`biome check scripts/check-swagger-rules.js package.json`,
		`npm run lint:check`,
	} {
		repo := t.TempDir()
		if err := os.WriteFile(filepath.Join(repo, "package.json"), []byte(`{"scripts":{"lint:check":"biome check --staged --no-errors-on-unmatched"}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Repo:                repo,
			Tool:                "bash",
			Command:             command,
			EnforceStagedChecks: true,
		})
		if got.Decision != "allow" {
			t.Fatalf("expected scoped staged check to be allowed: %q -> %+v", command, got)
		}
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

func TestBuildNextActionJudgementTriggerReportsRecommendedChoiceFacts(t *testing.T) {
	message := strings.Join([]string{
		"구현을 마쳤습니다.",
		"선택지:",
		"1. 진행: 다음 단계 테스트를 추가하고 구현을 계속합니다. (추천)",
		"2. 축소 진행: 일부만 먼저 검증합니다.",
		"3. 보류: 현재 상태로 멈추고 사용자 확인을 기다립니다.",
	}, "\n")
	result := BuildNextActionJudgementTrigger(message)
	if !result.OK {
		t.Fatalf("expected ok result, got %+v", result)
	}
	if !result.ShouldReenterAgent {
		t.Fatalf("expected next-action facts to re-enter the main agent, got %+v", result)
	}
	if !result.ChoicesFound || result.ChoiceCount != 3 {
		t.Fatalf("expected three observed choices, got %+v", result)
	}
	if result.RecommendedCount != 1 || result.RecommendedIndex != 1 || result.RecommendedText == "" {
		t.Fatalf("expected exactly one recommended choice fact, got %+v", result)
	}
	for _, candidate := range result.Candidates {
		if candidate.Score != 0 || candidate.Destructive {
			t.Fatalf("trigger must not score choices or emit destructive verdicts, got %+v", result)
		}
	}
}

func TestBuildNextActionJudgementTriggerReportsDestructiveTextAsFactOnly(t *testing.T) {
	message := strings.Join([]string{
		"리뷰가 통과했습니다.",
		"선택지:",
		"1. 진행: PR을 머지하고 이슈를 닫습니다. (추천)",
		"2. 보류: 추가 확인을 기다립니다.",
		"3. 축소: 일부만 merge 합니다.",
	}, "\n")
	result := BuildNextActionJudgementTrigger(message)
	if !result.ShouldReenterAgent {
		t.Fatalf("expected hook to relay facts to the main agent, got %+v", result)
	}
	for _, candidate := range result.Candidates {
		if candidate.Destructive || candidate.Score != 0 {
			t.Fatalf("hook trigger must not judge destructive text or score choices, got %+v", result)
		}
	}
}

func TestBuildNextActionJudgementTriggerReportsMissingRecommendationAsFact(t *testing.T) {
	message := strings.Join([]string{
		"선택지:",
		"1. 해석 A로 구현합니다.",
		"2. 해석 B로 구현합니다.",
		"3. 해석 C로 구현합니다.",
	}, "\n")
	result := BuildNextActionJudgementTrigger(message)
	if !result.ShouldReenterAgent || result.RecommendedCount != 0 {
		t.Fatalf("expected missing recommendation to be relayed as facts, got %+v", result)
	}
}

func TestBuildNextActionJudgementTriggerReportsMultipleRecommendationsAsFact(t *testing.T) {
	message := strings.Join([]string{
		"선택지:",
		"1. 진행: 구현합니다. (추천)",
		"2. 검증: 테스트합니다. (추천)",
		"3. 보류: 멈춥니다.",
	}, "\n")
	result := BuildNextActionJudgementTrigger(message)
	if !result.ShouldReenterAgent || result.RecommendedCount != 2 {
		t.Fatalf("expected multiple recommendations to be relayed as facts, got %+v", result)
	}
}

func TestBuildNextActionJudgementTriggerDoesNotParseExplanatoryNumberedText(t *testing.T) {
	message := strings.Join([]string{
		"설명입니다.",
		"1. Stop hook이 먼저 정적 휴리스틱으로 자동진행 후보를 판정합니다.",
		"2. 메인 에이전트는 실행 여부만 판단합니다.",
		"3. `agent-harness`가 추천 선택지를 분석해서 자동진행 후보라고 판단합니다.",
	}, "\n")
	result := BuildNextActionJudgementTrigger(message)
	if result.ShouldReenterAgent || result.ChoicesFound {
		t.Fatalf("numbered explanation without 선택지 header must not trigger, got %+v", result)
	}
}

func TestBuildNextActionJudgementTriggerNoChoicesDoesNotTrigger(t *testing.T) {
	result := BuildNextActionJudgementTrigger("작업을 완료했습니다.")
	if result.ShouldReenterAgent || result.ChoicesFound {
		t.Fatalf("message without choices must not trigger, got %+v", result)
	}
}

func guardRepoWithCycle(t *testing.T, branch string, phase IssueOpsPhase) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/"+branch+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	if phase != IssueOpsPhaseProblem {
		if _, err := AdvanceIssueOpsPhase(IssueOpsStateRoot(), rec.ID, string(phase)); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func linkIssueOpsBranchEvidenceForTest(t *testing.T, repo, branch string) {
	t.Helper()
	id := newIssueOpsID(repo, branch)
	before, err := ReadIssueOps(IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	issueURL := "https://github.com/example/repo/issues/1"
	if _, err := LinkIssueOpsIssue(IssueOpsStateRoot(), id, issueURL); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIssueOpsBranch(IssueOpsStateRoot(), id, IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     issueURL,
		Branch:       branch,
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	if before.Phase != IssueOpsPhaseProblem && before.Phase != IssueOpsPhasePlan {
		if _, err := AdvanceIssueOpsPhase(IssueOpsStateRoot(), id, string(before.Phase)); err != nil {
			t.Fatal(err)
		}
	}
}

func markIssueOpsPRPhaseForTest(t *testing.T, repo, branch string) {
	t.Helper()
	id := newIssueOpsID(repo, branch)
	record, err := ReadIssueOps(IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = IssueOpsPhasePR
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
}

func TestWorktreeGuardBlocksSourceEditWhenImplementCycleHasNoLinkedWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "development", IssueOpsPhaseImplement)
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if res.Decision != "block" || !strings.Contains(res.Reason, "requires a linked isolated worktree") {
		t.Fatalf("unlinked implement cycle should block source checkout edits, got %+v", res)
	}
}

func TestWorktreeGuardAllowsWorktreeAddBeforeLinkWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "development", IssueOpsPhaseImplement)
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Bash", Command: "git worktree add ../repo.worktrees/development origin/development", EnforceWorktree: true,
	})
	if res.Decision != "allow" {
		t.Fatalf("worktree preparation command should pass before link-worktree, got %+v", res)
	}
}

func TestWorktreeGuardBlocksLocalCheckoutOfIssueOpsBranch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "main", IssueOpsPhasePlan)
	rec, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "feature/issue-work"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AdvanceIssueOpsPhase(IssueOpsStateRoot(), rec.ID, string(IssueOpsPhaseImplement)); err != nil {
		t.Fatal(err)
	}

	blocked := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Bash", Command: "git checkout -b feature/issue-work", EnforceWorktree: true,
	})
	if blocked.Decision != "block" || !strings.Contains(blocked.Reason, "must not be checked out in the source checkout") {
		t.Fatalf("local checkout of IssueOps branch should block: %+v", blocked)
	}
}

func TestWorktreeGuardBlocksSourceEditWhenCycleHasLinkedWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "feat/x", IssueOpsPhaseImplement)
	id := newIssueOpsID(repo, "feat/x")
	linked := filepath.Join(filepath.Dir(repo), "repo.worktrees", "feat-x")
	linkIssueOpsBranchEvidenceForTest(t, repo, "feat/x")
	if err := os.MkdirAll(linked, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), id, linked); err != nil {
		t.Fatal(err)
	}
	blocked := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if blocked.Decision != "block" {
		t.Fatalf("source-checkout edit should block when an exact worktree is linked, got %+v", blocked)
	}
	wtTarget := filepath.Join(linked, "internal", "x.go")
	allowed := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{wtTarget}, EnforceWorktree: true,
	})
	if allowed.Decision == "block" {
		t.Fatalf("edit targeting the isolated worktree should pass, got %+v", allowed)
	}
}

func TestWorktreeGuardBlocksSourceEditDuringAISlopClean(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "feat/x", IssueOpsPhaseImplement)
	id := newIssueOpsID(repo, "feat/x")
	linked := filepath.Join(filepath.Dir(repo), "repo.worktrees", "feat-x")
	if err := os.MkdirAll(linked, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsIssue(IssueOpsStateRoot(), id, "https://github.com/example/repo/issues/1"); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIssueOpsBranch(IssueOpsStateRoot(), id, IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     "https://github.com/example/repo/issues/1",
		Branch:       "feat/x",
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsPlan(IssueOpsStateRoot(), id, "plans/demo.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), id, linked); err != nil {
		t.Fatal(err)
	}
	if _, err := AdvanceIssueOpsPhase(IssueOpsStateRoot(), id, string(IssueOpsPhaseAISlopClean)); err != nil {
		t.Fatal(err)
	}
	blocked := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if blocked.Decision != "block" || !strings.Contains(blocked.Reason, "linked IssueOps worktree") {
		t.Fatalf("source-checkout edit should block during ai-slop-clean, got %+v", blocked)
	}
}

func TestWorktreeGuardBlocksOtherWorktreeWhenCycleHasExactWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "feat/x", IssueOpsPhaseImplement)
	id := newIssueOpsID(repo, "feat/x")
	expected := filepath.Join(filepath.Dir(repo), "repo.worktrees", "feat-x")
	linkIssueOpsBranchEvidenceForTest(t, repo, "feat/x")
	if err := os.MkdirAll(expected, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), id, expected); err != nil {
		t.Fatal(err)
	}

	other := filepath.Join(filepath.Dir(repo), "repo.worktrees", "feat-y", "internal", "x.go")
	blocked := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{other}, EnforceWorktree: true,
	})
	if blocked.Decision != "block" || !strings.Contains(blocked.Reason, "linked IssueOps worktree") {
		t.Fatalf("other worktree edit should block when exact worktree is linked: %+v", blocked)
	}

	allowed := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{filepath.Join(expected, "internal", "x.go")}, EnforceWorktree: true,
	})
	if allowed.Decision != "allow" {
		t.Fatalf("linked IssueOps worktree edit should pass: %+v", allowed)
	}
}

func TestWorktreeGuardIgnoresLinkedCycleFromOtherBranch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "main", IssueOpsPhaseProblem)
	rec, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "feat/x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AdvanceIssueOpsPhase(IssueOpsStateRoot(), rec.ID, string(IssueOpsPhaseImplement)); err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(filepath.Dir(repo), "repo.worktrees", "feat-x")
	linkIssueOpsBranchEvidenceForTest(t, repo, "feat/x")
	if err := os.MkdirAll(expected, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), rec.ID, expected); err != nil {
		t.Fatal(err)
	}

	blocked := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{filepath.Join(repo, "internal", "x.go")}, EnforceWorktree: true,
	})
	if blocked.Decision != "allow" {
		t.Fatalf("other branch linked worktree should not lock current checkout: %+v", blocked)
	}

	allowed := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{filepath.Join(expected, "internal", "x.go")}, EnforceWorktree: true,
	})
	if allowed.Decision != "allow" {
		t.Fatalf("linked issue worktree edit should pass even when source checkout is main: %+v", allowed)
	}
}

func TestWorktreeGuardBlocksOtherWorktreeWhenCurrentBranchCycleIsUnlinked(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "main", IssueOpsPhaseImplement)
	rec, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "feat/x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AdvanceIssueOpsPhase(IssueOpsStateRoot(), rec.ID, string(IssueOpsPhaseImplement)); err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(filepath.Dir(repo), "repo.worktrees", "feat-x")
	linkIssueOpsBranchEvidenceForTest(t, repo, "feat/x")
	if err := os.MkdirAll(expected, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), rec.ID, expected); err != nil {
		t.Fatal(err)
	}

	other := filepath.Join(filepath.Dir(repo), "repo.worktrees", "feat-y", "internal", "x.go")
	blocked := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{other}, EnforceWorktree: true,
	})
	if blocked.Decision != "block" || !strings.Contains(blocked.Reason, "requires a linked isolated worktree") {
		t.Fatalf("unlinked current branch cycle should block unrelated worktree edits: %+v", blocked)
	}
}

func TestWorktreeGuardNoBlockWithoutCycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644)
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if res.Decision == "block" {
		t.Fatalf("no cycle for this branch should not block, got %+v", res)
	}
}

func TestWorktreeGuardNoBlockWhenCycleDone(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "feat/x", IssueOpsPhaseImplement)
	id := newIssueOpsID(repo, "feat/x")
	markIssueOpsPRPhaseForTest(t, repo, "feat/x")
	if _, err := AdvanceIssueOpsPhase(IssueOpsStateRoot(), id, string(IssueOpsPhaseDone)); err != nil {
		t.Fatal(err)
	}
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if res.Decision == "block" {
		t.Fatalf("done cycle should release the source checkout, got %+v", res)
	}
}

func TestWorktreeGuardNoBlockInPlanningPhase(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "feat/x", IssueOpsPhaseGrill)
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if res.Decision == "block" {
		t.Fatalf("planning phase expects no worktree yet; should not block, got %+v", res)
	}
}

func TestWorktreeGuardIgnoresOtherBranchCycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	// Active implement cycle for a different branch must not lock edits on main.
	repo := guardRepoWithCycle(t, "feat/other", IssueOpsPhaseImplement)
	_ = os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644)
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if res.Decision == "block" {
		t.Fatalf("a cycle for a different branch must not lock the current branch, got %+v", res)
	}
}

func TestWorktreeGuardIgnoresMismatchedWorktreePlanBranch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "development", IssueOpsPhasePlan)
	recordID := newIssueOpsID(repo, "development")

	worktree := filepath.Join(filepath.Dir(repo), "repo.worktrees", "bugfix-2361")
	gitdir := filepath.Join(repo, ".git", "worktrees", "bugfix-2361")
	if err := os.MkdirAll(filepath.Join(worktree, "docs", "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(gitdir, 0o755); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(worktree, gitdir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+rel+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitdir, "HEAD"), []byte("ref: refs/heads/bugfix/2361\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkIssueOpsBranchEvidenceForTest(t, repo, "development")
	if _, err := LinkIssueOpsPlan(IssueOpsStateRoot(), recordID, filepath.Join(worktree, "docs", "plans", "2361.md")); err != nil {
		t.Fatal(err)
	}

	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Edit", Paths: []string{repo + "/internal/x.go"}, EnforceWorktree: true,
	})
	if res.Decision == "block" {
		t.Fatalf("mismatched worktree plan branch should not lock source checkout, got %+v", res)
	}
}

func TestWorktreeGuardAllowsTempFileBashWrites(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "feat/x", IssueOpsPhaseImplement)
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Bash", Command: "cat > /tmp/mr-body.md <<EOF\nbody\nEOF", EnforceWorktree: true,
	})
	if res.Decision == "block" {
		t.Fatalf("temp file bash writes should not be treated as source checkout edits, got %+v", res)
	}
}

func TestWorktreeGuardBlocksBashCommandThatChangesIntoUnlinkedWorktree(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "feat/x", IssueOpsPhaseImplement)
	worktree := filepath.Join(filepath.Dir(repo), "repo.worktrees", "feat-x")
	res := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo: repo, Tool: "Bash", Command: "cd " + worktree + " && printf body > /tmp/mr-body.md", EnforceWorktree: true,
	})
	if res.Decision != "block" || !strings.Contains(res.Reason, "requires a linked isolated worktree") {
		t.Fatalf("bash command scoped to unlinked worktree should block, got %+v", res)
	}
}

func TestActiveIssueOpsCycleForBranchIsDeterministicAndReleasesOnDone(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, ok := ActiveIssueOpsCycleForBranch(repo, "main"); ok {
		t.Fatalf("no cycle yet")
	}
	first, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	// Re-starting the same (repo, branch) must resume the same record, not duplicate.
	second, err := StartIssueOps(IssueOpsStateRoot(), IssueOpsStartRequest{Repo: repo, Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("start must be idempotent per (repo, branch): %s != %s", first.ID, second.ID)
	}
	if _, ok := ActiveIssueOpsCycleForBranch(repo, "main"); !ok {
		t.Fatalf("active cycle should be found")
	}
	if _, ok := ActiveIssueOpsCycleForBranch(repo, "other"); ok {
		t.Fatalf("a different branch must not match")
	}
	markIssueOpsPRPhaseForTest(t, repo, "main")
	if _, err := AdvanceIssueOpsPhase(IssueOpsStateRoot(), first.ID, string(IssueOpsPhaseDone)); err != nil {
		t.Fatal(err)
	}
	if _, ok := ActiveIssueOpsCycleForBranch(repo, "main"); ok {
		t.Fatalf("done cycle must not be reported active")
	}
}

func TestGitBranchFromHeadResolvesRelativeLinkedWorktreeGitdir(t *testing.T) {
	base := t.TempDir()
	// Simulate a linked worktree: <base>/wt/.git is a file pointing to a relative
	// gitdir, and HEAD lives under that resolved gitdir.
	wt := filepath.Join(base, "repo.worktrees", "feat-x")
	gitdir := filepath.Join(base, "repo", ".git", "worktrees", "feat-x")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(gitdir, 0o755); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(wt, gitdir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+rel+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitdir, "HEAD"), []byte("ref: refs/heads/feat/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := gitBranchFromHead(wt); got != "feat/x" {
		t.Fatalf("expected branch feat/x from relative linked-worktree gitdir, got %q", got)
	}
}
