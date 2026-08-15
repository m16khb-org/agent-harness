package vcsissue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHookPreToolUseBlocksPlanLinkSectionInIssueBody(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	bodyFile := filepath.Join(repo, "body.md")
	if err := os.WriteFile(bodyFile, []byte("## Problem\n\n문제 설명입니다.\n\nPlan Link:\n\nTBD\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"cwd":        repo,
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": `gh issue create --title "이슈" --body-file body.md --label bug --assignee sample`},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runPreToolUseCapture(t, string(payload), "--enforce-vcs-issue-linking", "--json")
	if obj["decision"] != "block" {
		t.Fatalf("expected Plan Link section to be blocked, got %+v", obj)
	}
	if reason, _ := obj["reason"].(string); !strings.Contains(reason, "Plan Link") {
		t.Fatalf("expected Plan Link reason, got %q", reason)
	}
}

func TestRunHookPreToolUseBlocksGitLabRelatedIssuesBodySection(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": "glab issue create --title 이슈 --description \"## Problem\n\n설명\n\n관련 이슈\n\n- #1\"",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runPreToolUseCapture(t, string(payload), "--enforce-vcs-issue-linking", "--json")
	if obj["decision"] != "block" {
		t.Fatalf("expected GitLab Related Issues body section to be blocked, got %+v", obj)
	}
	if reason, _ := obj["reason"].(string); !strings.Contains(reason, "linked items") {
		t.Fatalf("expected GitLab linked items reason, got %q", reason)
	}
}

func TestRunHookPreToolUseBlocksStructuredGitLabRelatedIssuesBodySection(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__create_issue",
		"tool_input": map[string]any{
			"title":       "한국어 제목입니다 충분합니다",
			"description": "관련 이슈\n- #1\n문제 설명입니다. 한국어 본문을 충분히 작성합니다.",
			"labels":      []any{"bug"},
			"assignee":    "sample",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runPreToolUseCapture(t, string(payload), "--enforce-korean-remote-artifacts", "--enforce-vcs-issue-linking", "--json")
	if obj["decision"] != "block" {
		t.Fatalf("expected structured GitLab Related Issues body section to be blocked, got %+v", obj)
	}
	if reason, _ := obj["reason"].(string); !strings.Contains(reason, "linked items") {
		t.Fatalf("expected GitLab linked items reason, got %q", reason)
	}
}

func TestRunHookPreToolUseAllowsGitHubRelatedIssuesBodySection(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	bodyFile := filepath.Join(repo, "body.md")
	if err := os.WriteFile(bodyFile, []byte("## Problem\n\n설명입니다.\n\n## Related Issues\n\n- #1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"cwd":        repo,
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": `gh issue create --title "이슈" --body-file body.md --label bug --assignee sample`},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runPreToolUseCapture(t, string(payload), "--enforce-vcs-issue-linking", "--json")
	if obj["decision"] == "block" {
		t.Fatalf("GitHub body references are valid and must not be blocked, got %+v", obj)
	}
}
