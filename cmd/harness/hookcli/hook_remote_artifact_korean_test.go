package hookcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHookPreToolUseEnforcesKoreanRemoteArtifacts(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": `gh pr create --title "Document split and IssueOps guardrails" --body "Summary Changes Verification Risk"`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-korean-remote-artifacts"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected English PR artifact to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "IssueOps remote artifact gate failed") {
		t.Fatalf("expected Korean remote artifact gate reason, got %q", reason)
	}
}

func TestRunHookPreToolUseAllowsRemoteArtifactEditWithoutBody(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": `glab issue edit 123 --add-label bug`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-korean-remote-artifacts", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("remote artifact edit without body should not require title/body text, got %+v", obj)
	}
}

func TestRunHookPreToolUseInspectsGitLabDescriptionFile(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	bodyFile := filepath.Join(repo, "issue.md")
	if err := os.WriteFile(bodyFile, []byte("This issue body is English prose and should be blocked before remote creation.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": `glab issue create --title "Remote artifact check" --description-file issue.md --label bug --assignee habin`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-korean-remote-artifacts", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected English GitLab description file to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "IssueOps remote artifact gate failed") {
		t.Fatalf("expected Korean remote artifact gate reason, got %q", reason)
	}
}

func TestRunHookPreToolUseInspectsInlineHereDocBodyFile(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	command := `body=$(mktemp)
cat > "$body" <<'EOF'
## 요약
IssueOps 라이프사이클 감사용 임시 PR입니다. 실제 원격 PR label과 assignee 검증을 확인합니다.
EOF
gh pr create --title "IssueOps 라이프사이클 감사용 임시 PR" --body-file "$body" --label bug --assignee m16khb`
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": command,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-korean-remote-artifacts", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected inline here-doc body-file to be inspected and allowed, got %+v", obj)
	}
}

func TestRunHookPreToolUseBlocksEnglishMCPRemoteArtifact(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	for _, tc := range []struct {
		name  string
		tool  string
		input map[string]any
	}{
		{
			name: "gitlab issue",
			tool: "mcp__glab__create_issue",
			input: map[string]any{
				"title":       "Investigate routing regression",
				"description": "The implementation should verify the failing route and update the service contract.",
				"labels":      []any{"bug"},
				"assignee":    "@me",
			},
		},
		{
			name: "gitlab mr",
			tool: "mcp__gitlab__create_merge_request",
			input: map[string]any{
				"title":       "Fix adult routing regression",
				"description": "This merge request updates the service and documents the verification evidence.",
				"labels":      []any{"bug"},
				"assignee":    "@me",
			},
		},
		{
			name: "github pr",
			tool: "mcp__github__create_pull_request",
			input: map[string]any{
				"title":    "Fix adult routing regression",
				"body":     "This pull request updates the service and documents the verification evidence.",
				"labels":   []any{"bug"},
				"assignee": "@me",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(map[string]any{
				"cwd":        repo,
				"tool_name":  tc.tool,
				"tool_input": tc.input,
			})
			if err != nil {
				t.Fatal(err)
			}
			obj := runHookCapture(t, string(payload), func() error {
				return runHookPreToolUse([]string{"--enforce-korean-remote-artifacts", "--json"})
			})
			if obj["decision"] != "block" {
				t.Fatalf("expected English MCP remote artifact creation to be blocked, got %+v", obj)
			}
			if reason, _ := obj["reason"].(string); !strings.Contains(reason, "IssueOps remote artifact gate failed") {
				t.Fatalf("expected Korean gate reason, got %q", reason)
			}
		})
	}
}

func TestRunHookPreToolUseRemoteArtifactGateIgnoresCodeGraphQueryText(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__codegraph__codegraph_explore",
		"tool_input": map[string]any{
			"query": `glab mr create --title "IssueOps 담당자 검증" --description "라벨과 담당자 누락을 설명하는 탐색 문자열"`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-korean-remote-artifacts", "--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected CodeGraph query text to bypass remote artifact gates, got %+v", obj)
	}
}

func TestRunHookPreToolUseAllowsKoreanGitLabMCPRemoteArtifact(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__create_issue",
		"tool_input": map[string]any{
			"title":       "성인 라우팅 회귀 원인 조사",
			"description": "문제 배경과 재현 경로를 정리하고, 변경 후 검증 명령과 운영 확인 결과를 이슈 본문에 기록합니다.",
			"labels":      []any{"bug"},
			"assignee":    "@me",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-korean-remote-artifacts", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected Korean GitLab MCP issue creation to be allowed, got %+v", obj)
	}
}
