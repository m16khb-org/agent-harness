package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunHookPreToolUseAllowsBoundedReadOnlySequence(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := filepath.Join(t.TempDir(), "agent-harness")
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycle := createLinkedIssueOpsWorktree(t, source, "75-readonly-sequence-hook")
	_ = activateIssueOpsHookExecution(t, cycle.id)
	commands := []struct {
		name    string
		command string
	}{
		{
			name: "CodeGraph와 Git 관찰",
			command: `if [ -d .codegraph ]; then printf 'codegraph-present\n'; else printf 'codegraph-absent\n'; fi
git status --short
git branch --show-current
git rev-parse HEAD
git diff --stat
git diff --cached --stat`,
		},
		{
			name: "여러 파일 관찰",
			command: `sed -n '1,126p' internal/core/issueops/testdata/leasevertical/application/release.go
sed -n '1,130p' internal/core/issueops/testdata/leasevertical/domain/release.go
sed -n '1,383p' internal/core/issueops/testdata/leasevertical/contract/record.go`,
		},
		{
			name:    "축약 context 옵션을 쓴 rg 관찰",
			command: `rg -n -A5 'NewReleaseService\(' internal/core/issueops/execution_lease_differential_test.go internal/core/issueops/testdata/leasevertical/application/release.go`,
		},
		{
			name:    "Turing JSON 구문 검증",
			command: "jq empty .agent-harness/turing/issueops-v1-0d097a7cae7456be.json",
		},
		{
			name:    "프로젝트 문서 전체 관찰",
			command: `sed -n '1,$p' .agent-harness/CONVENTIONS.md`,
		},
		{
			name:    "성공 조건으로 연결한 Git 관찰",
			command: "pwd && git status --short && git diff --cached --check",
		},
		{
			name:    "rg 결과의 bounded head 관찰",
			command: `rg -n 'ai-slop-clean|AISlopCleanCategories|category' cmd/harness/issueopscli internal/core/issueops | head -160`,
		},
	}

	for _, testCase := range commands {
		t.Run(testCase.name, func(t *testing.T) {
			raw, err := json.Marshal(map[string]any{
				"cwd": cycle.path, "host": "codex", "session_id": "observer-session",
				"tool_name": "Bash", "tool_input": map[string]any{"command": testCase.command},
			})
			if err != nil {
				t.Fatal(err)
			}
			got := runHookCapture(t, string(raw), func() error {
				return runHookPreToolUse([]string{"--host", "codex", "--enforce-worktree", "--json"})
			})
			if got["decision"] != "allow" {
				t.Fatalf("hook은 정적으로 판정 가능한 읽기 전용 시퀀스를 허용해야 한다: %+v", got)
			}
		})
	}
}
