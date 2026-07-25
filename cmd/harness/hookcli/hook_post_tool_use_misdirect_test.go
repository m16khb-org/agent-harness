package hookcli

import (
	"os"
	"path/filepath"
	"testing"
)

// #95 실측 사고 형상: canonical worktree 안의 파일을 편집했는데 tool_response의
// diff 텍스트("cmd/….go")가 경로로 오추출되어 base(소스 체크아웃) 기준으로
// 해석되면 SourceCheckoutMisdirectWarning이 오탐된다. 추출→해석→판정 경로
// 전체를 훅 레벨에서 봉인한다.
func TestRunHookPostToolUseToolResponseTextDoesNotTriggerSourceMisdirect(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, repo, "95-hook-response-noise")
	activateIssueOpsHookExecution(t, cycle.id)
	target := filepath.Join(cycle.path, "plan.md")
	raw := runHookCapture(t, `{
  "cwd": "`+repo+`",
  "tool_name": "Edit",
  "tool_input": {"file_path": "`+target+`"},
  "tool_response": {
    "filePath": "`+target+`",
    "structuredPatch": ["+see cmd/harness/issueopscli/issueops.go"]
  }
}`, func() error {
		return runHookPostToolUse([]string{"--json"})
	})
	if warning, _ := raw["misdirect_warning"].(string); warning != "" {
		t.Fatalf("worktree edit must not be misread as source-checkout mutation via tool_response text: %q", warning)
	}
}
