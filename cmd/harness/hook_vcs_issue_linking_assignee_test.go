package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunHookPreToolUseEnforcesRemoteCreateAssignee(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": `glab mr create --title "IssueOps 담당자 검증" --description "라벨은 있지만 담당자 없는 MR 생성을 막습니다." --label bug`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected missing assignee to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "assignee") {
		t.Fatalf("expected assignee reason, got %q", reason)
	}
}

func TestRunHookPreToolUseStructuredRemoteCreateReadsGlabFlagsAssignee(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__glab_mr_create",
		"tool_input": map[string]any{
			"flags": map[string]any{
				"title":             "IssueOps 담당자 검증",
				"description":       "이슈 라벨 복사와 담당자를 함께 지정합니다.",
				"copy_issue_labels": true,
				"assignee":          []any{"m16khb"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected structured labeled and assigned MR create to be allowed, got %+v", obj)
	}
}

func TestRunHookPreToolUseStructuredRemoteCreateStillRequiresAssignee(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__glab_mr_create",
		"tool_input": map[string]any{
			"flags": map[string]any{
				"title":             "IssueOps 담당자 검증",
				"description":       "이슈 라벨 복사 옵션이 있어도 담당자는 필요합니다.",
				"copy_issue_labels": true,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected structured MR create without assignee to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "assignee") {
		t.Fatalf("expected assignee reason, got %q", reason)
	}
}

func TestRunHookPreToolUseStructuredGlabMRForRequiresAssignee(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__glab_mr_for",
		"tool_input": map[string]any{
			"args": []any{"2385"},
			"flags": map[string]any{
				"with_labels": true,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected structured glab mr for without assignee to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "assignee") {
		t.Fatalf("expected assignee reason, got %q", reason)
	}
}

func TestRunHookPreToolUseStructuredGlabMRForAllowsAssignee(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__glab_mr_for",
		"tool_input": map[string]any{
			"args": []any{"2385"},
			"flags": map[string]any{
				"with_labels": true,
				"assignee":    "100",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected structured glab mr for with assignee to be allowed, got %+v", obj)
	}
	if command, _ := obj["command"].(string); !strings.Contains(command, `glab mr for "2385"`) {
		t.Fatalf("expected structured glab mr for to preserve positional issue argument, got %q", command)
	}
}

func TestRunHookPreToolUseStructuredGlabMRForAllowsNumericAssigneeID(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__glab_mr_for",
		"tool_input": map[string]any{
			"args": []any{"2385"},
			"flags": map[string]any{
				"with_labels": true,
				"assignee_id": 100,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected structured glab mr for with numeric assignee_id to be allowed, got %+v", obj)
	}
	if command, _ := obj["command"].(string); !strings.Contains(command, `glab mr for "2385"`) || !strings.Contains(command, `--assignee-id "100"`) {
		t.Fatalf("expected structured glab mr for to preserve issue and assignee id, got %q", command)
	}
}

func TestRunHookPreToolUseStructuredGlabMRForRejectsUsernameAssignee(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__glab_mr_for",
		"tool_input": map[string]any{
			"args": []any{"2385"},
			"flags": map[string]any{
				"with_labels": true,
				"assignee":    "m16khb",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected structured glab mr for username assignee to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "numeric assignee") {
		t.Fatalf("expected numeric assignee reason, got %q", reason)
	}
}

func TestRunHookPreToolUseAllowsIssueBasedGitLabMRWithCombinedGates(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "Bash",
		"tool_input": map[string]any{
			"command": `glab mr for 2385 --with-labels --assignee 100`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-korean-remote-artifacts", "--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "allow" {
		t.Fatalf("expected issue-based GitLab MR with labels and numeric assignee to be allowed, got %+v", obj)
	}
}

func TestRunHookPreToolUseStructuredRelatedIssueMRRequiresNumericAssignee(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__glab_mr_create",
		"tool_input": map[string]any{
			"flags": map[string]any{
				"related_issue":     "2385",
				"copy_issue_labels": true,
				"assignee":          "m16khb",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := runHookCapture(t, string(payload), func() error {
		return runHookPreToolUse([]string{"--enforce-vcs-issue-linking", "--json"})
	})
	if obj["decision"] != "block" {
		t.Fatalf("expected structured related-issue MR with username assignee to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	if !strings.Contains(reason, "numeric assignee") {
		t.Fatalf("expected numeric assignee reason, got %q", reason)
	}
}
