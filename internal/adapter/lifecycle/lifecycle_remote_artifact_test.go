package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lifecyclecontract "agent-harness/internal/contract/lifecycle"
)

func TestPreToolUseKoreanRemoteArtifactGateBlocksEnglishPR(t *testing.T) {
	got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
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
	got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
		Repo:                repo,
		Tool:                "bash",
		Command:             `gh pr create --title "문서 분할과 IssueOps guardrail 추가" --body-file pr-body.md`,
		EnforceKoreanRemote: true,
	})
	if got.Decision != "allow" {
		t.Fatalf("expected Korean PR artifact to be allowed: %+v", got)
	}
}

func TestPreToolUseKoreanRemoteArtifactGateAllowsInlineHereDocBodyFile(t *testing.T) {
	got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
		Repo: t.TempDir(),
		Tool: "bash",
		Command: `body=$(mktemp)
cat > "$body" <<'EOF'
## 요약
IssueOps 라이프사이클 감사용 임시 PR입니다. 실제 원격 PR label과 assignee 검증을 확인합니다.
EOF
gh pr create --title "IssueOps 라이프사이클 감사용 임시 PR" --body-file "$body" --label bug --assignee m16khb`,
		EnforceKoreanRemote: true,
	})
	if got.Decision != "allow" {
		t.Fatalf("expected inline here-doc body-file to be inspected and allowed: %+v", got)
	}
}

func TestPreToolUseKoreanRemoteArtifactGateBlocksEnglishInlineHereDocBodyFile(t *testing.T) {
	got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
		Repo: t.TempDir(),
		Tool: "bash",
		Command: `body=$(mktemp)
cat > "$body" <<'EOF'
Summary
This pull request uses English prose and should still be blocked before remote creation.
EOF
gh pr create --title "IssueOps audit temporary PR" --body-file "$body" --label bug --assignee m16khb`,
		EnforceKoreanRemote: true,
	})
	if got.Decision != "block" || !strings.Contains(got.Reason, "IssueOps remote artifact gate failed") {
		t.Fatalf("expected English inline here-doc body-file to be inspected and blocked: %+v", got)
	}
}

func TestPreToolUseRemoteArtifactGateAllowsHelpCommands(t *testing.T) {
	for _, command := range []string{
		`gh issue create --help`,
		`gh pr create -h`,
		`glab issue create --help`,
		`glab mr create -h`,
	} {
		got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
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

func TestPreToolUseRemoteArtifactGateIgnoresCodeGraphQueryText(t *testing.T) {
	got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
		Repo:                t.TempDir(),
		Tool:                "mcp__codegraph__codegraph_explore",
		Command:             `glab mr create --title "IssueOps 담당자 검증" --description "라벨과 담당자 누락을 설명하는 탐색 문자열"`,
		EnforceKoreanRemote: true,
		EnforceVCSLinking:   true,
	})
	if got.Decision != "allow" {
		t.Fatalf("expected CodeGraph query text to bypass remote artifact gates: %+v", got)
	}
}

func TestPreToolUseKoreanRemoteArtifactGateAllowsGitLabIssueBasedMR(t *testing.T) {
	for _, command := range []string{
		`glab mr for 2385 --with-labels --assignee 100`,
		`glab mr create --related-issue 2385 --copy-issue-labels --assignee-id 100`,
	} {
		got := BuildLifecyclePreToolUseDecision(lifecyclecontract.HookToolUseLifecycleRequest{
			Repo:                t.TempDir(),
			Tool:                "bash",
			Command:             command,
			EnforceKoreanRemote: true,
			EnforceVCSLinking:   true,
		})
		if got.Decision != "allow" {
			t.Fatalf("expected issue-based GitLab MR create to be allowed with labels and numeric assignee: %q -> %+v", command, got)
		}
	}
}
