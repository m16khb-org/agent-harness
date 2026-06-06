package core

import (
	"strings"
	"testing"
)

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
		`glab mr for 2385 --with-labels`,
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

func TestPreToolUseVCSLinkingBlocksGitLabPlaceholderAssignee(t *testing.T) {
	for _, command := range []string{
		`glab issue create --title "IssueOps 담당자 검증" --description "GitLab 이슈 담당자는 실제 사용자명이어야 합니다." --label bug --assignee @me`,
		`glab mr create --title "IssueOps 담당자 검증" --description "GitLab MR 담당자는 실제 사용자명이어야 합니다." --label bug --assignee self`,
		`glab mr for 2385 --with-labels --assignee m16khb`,
	} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
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
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
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
		`gh pr create --title "IssueOps 라벨 검증" --body "라벨과 담당자를 함께 지정합니다." -l bug -a habin`,
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

func TestPreToolUseGitLabRelatedIssueMRRequiresNumericAssignee(t *testing.T) {
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:              t.TempDir(),
		Tool:              "bash",
		Command:           `glab mr create --related-issue 2385 --copy-issue-labels --assignee m16khb`,
		EnforceVCSLinking: true,
	})
	if got.Decision != "block" || !strings.Contains(got.Reason, "numeric assignee") {
		t.Fatalf("expected GitLab issue-based MR create to require numeric assignee id, got %+v", got)
	}
}
