package lifecycle

import (
	"os"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
	lifecyclecontract "agent-harness/internal/contract/lifecycle"
)

func TestPreToolUseVCSLinkingBlocksRemoteCreateWithoutLabels(t *testing.T) {
	got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
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
		`glab mr for 2385 --with-labels`,
	} {
		got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
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

func TestPreToolUseVCSLinkingBlocksGitLabPlaceholderAssignee(t *testing.T) {
	for _, command := range []string{
		`glab issue create --title "IssueOps 담당자 검증" --description "GitLab 이슈 담당자는 실제 사용자명이어야 합니다." --label bug --assignee @me`,
		`glab mr create --title "IssueOps 담당자 검증" --description "GitLab MR 담당자는 실제 사용자명이어야 합니다." --label bug --assignee self`,
		`glab mr for 2385 --with-labels --assignee m16khb`,
	} {
		got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
			Repo:              t.TempDir(),
			Tool:              "bash",
			Command:           command,
			EnforceVCSLinking: true,
		})
		if got.Decision != "block" || !strings.Contains(got.Reason, "assignee") {
			t.Fatalf("expected placeholder or wrong GitLab assignee shape to be blocked: %q -> %+v", command, got)
		}
	}
}

func TestPreToolUseVCSLinkingBlocksGitHubPlaceholderAssignee(t *testing.T) {
	for _, command := range []string{
		`gh issue create --title "IssueOps 담당자 검증" --body "GitHub 이슈 담당자는 실제 사용자명이어야 합니다." --label bug --assignee @me`,
		`gh pr create --title "IssueOps 담당자 검증" --body "GitHub PR 담당자는 실제 사용자명이어야 합니다." --label bug --assignee self`,
	} {
		got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
			Repo:              t.TempDir(),
			Tool:              "bash",
			Command:           command,
			EnforceVCSLinking: true,
		})
		if got.Decision != "block" || !strings.Contains(got.Reason, "placeholder") {
			t.Fatalf("expected GitHub placeholder assignee to be blocked: %q -> %+v", command, got)
		}
	}
}

func TestPreToolUseVCSLinkingAllowsRemoteCreateWithLabelsAndAssignee(t *testing.T) {
	for _, command := range []string{
		`glab mr create --title "IssueOps 라벨 검증" --description "이슈 라벨을 복사해 MR 라벨 누락을 방지합니다." --label bug --assignee m16khb`,
		`glab mr create --title "IssueOps 라벨 검증" --description "이슈 라벨 복사와 담당자를 함께 지정합니다." --copy-issue-labels --assignee-id 100`,
		`glab mr for 2385 --with-labels --assignee 100`,
		`gh pr create --title "IssueOps 라벨 검증" --body "라벨과 담당자를 함께 지정합니다." -l bug -a sample`,
	} {
		got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
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

func TestPreToolUseGitLabRelatedIssueMRRequiresNumericAssignee(t *testing.T) {
	got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
		Repo:              t.TempDir(),
		Tool:              "bash",
		Command:           `glab mr create --related-issue 2385 --copy-issue-labels --assignee m16khb`,
		EnforceVCSLinking: true,
	})
	if got.Decision != "block" || !strings.Contains(got.Reason, "numeric assignee") {
		t.Fatalf("expected GitLab issue-based MR create to require numeric assignee id, got %+v", got)
	}
}

func TestPreToolUseVCSLinkingBlocksGitLabMRTargetBranchMismatch(t *testing.T) {
	repo := issueOpsPRTargetGuardRepo(t, "12-child", "2435-parent")

	got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
		Repo:              repo,
		Tool:              "bash",
		Command:           `glab mr create --title "IssueOps 대상 브랜치 검증" --description "자식 작업 MR은 기록된 부모 작업 브랜치로 합류해야 합니다." --source-branch 12-child --target-branch release/stg --label bug --assignee m16khb`,
		EnforceVCSLinking: true,
	})

	if got.Decision != "block" || !strings.Contains(got.Reason, "branch_prepare.base_branch") || !strings.Contains(got.Reason, "2435-parent") {
		t.Fatalf("expected mismatched GitLab MR target branch to be blocked, got %+v", got)
	}
}

func TestPreToolUseVCSLinkingBlocksGitHubPRBaseBranchMismatch(t *testing.T) {
	repo := issueOpsPRTargetGuardRepo(t, "12-child", "2435-parent")

	got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
		Repo:              repo,
		Tool:              "bash",
		Command:           `gh pr create --title "IssueOps 대상 브랜치 검증" --body "자식 작업 PR은 기록된 부모 작업 브랜치로 합류해야 합니다." --head 12-child --base main --label bug --assignee sample`,
		EnforceVCSLinking: true,
	})

	if got.Decision != "block" || !strings.Contains(got.Reason, "branch_prepare.base_branch") || !strings.Contains(got.Reason, "2435-parent") {
		t.Fatalf("expected mismatched GitHub PR base branch to be blocked, got %+v", got)
	}
}

func TestPreToolUseVCSLinkingAllowsPRTargetBranchFromBranchPrepare(t *testing.T) {
	repo := issueOpsPRTargetGuardRepo(t, "12-child", "2435-parent")

	got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
		Repo:              repo,
		Tool:              "bash",
		Command:           `gh pr create --title "IssueOps 대상 브랜치 검증" --body "자식 작업 PR이 기록된 부모 작업 브랜치로 합류합니다." --head 12-child --base 2435-parent --label bug --assignee sample`,
		EnforceVCSLinking: true,
	})

	if got.Decision != "allow" {
		t.Fatalf("expected matching PR base branch to be allowed, got %+v", got)
	}
}

func TestPreToolUseVCSLinkingBlocksMissingPRTargetBranchWhenBranchPrepareExists(t *testing.T) {
	repo := issueOpsPRTargetGuardRepo(t, "12-child", "2435-parent")

	got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
		Repo:              repo,
		Tool:              "bash",
		Command:           `gh pr create --title "IssueOps 대상 브랜치 검증" --body "자식 작업 PR이 기본 브랜치로 새지 않도록 target을 명시해야 합니다." --head 12-child --label bug --assignee sample`,
		EnforceVCSLinking: true,
	})

	if got.Decision != "block" || !strings.Contains(got.Reason, "must specify a target branch") || !strings.Contains(got.Reason, "2435-parent") {
		t.Fatalf("expected missing PR base branch to be blocked, got %+v", got)
	}
}

func issueOpsPRTargetGuardRepo(t *testing.T, branch, baseBranch string) string {
	t.Helper()
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if err := os.MkdirAll(repo+"/.git", 0o755); err != nil {
		t.Fatal(err)
	}
	record, err := StartIssueOps(IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsIssue(IssueOpsStateRoot(), record.ID, "https://github.com/example/repo/issues/12"); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareIssueOpsBranch(IssueOpsStateRoot(), record.ID, issueopscontract.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     "https://github.com/example/repo/issues/12",
		Branch:       branch,
		BaseBranch:   baseBranch,
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	return repo
}
