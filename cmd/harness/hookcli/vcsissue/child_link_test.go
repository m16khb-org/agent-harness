package vcsissue

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunHookPreToolUseBlocksStructuredGitLabIssueLinksForChildItems(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	payload, err := json.Marshal(map[string]any{
		"cwd":       repo,
		"tool_name": "mcp__glab__glab_api",
		"tool_input": map[string]any{
			"endpoint": "projects/1/issues/2/links",
			"method":   "POST",
			"flags": map[string]any{
				"target_issue_iid": 3,
				"link_type":        "relates_to",
				"note":             "child task under umbrella",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	obj := runPreToolUseCapture(t, string(payload), "--enforce-vcs-issue-linking", "--json")
	if obj["decision"] != "block" {
		t.Fatalf("expected structured GitLab issue links API child attempt to be blocked, got %+v", obj)
	}
	reason, _ := obj["reason"].(string)
	for _, want := range []string{"linked items and child items are different", "remote create-child"} {
		if !strings.Contains(reason, want) {
			t.Fatalf("expected reason to contain %q, got %q", want, reason)
		}
	}
}
