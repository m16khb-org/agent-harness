package issueops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIssueOpsMCPRecordsCompatibilityReview(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	configureIssueOpsMCPForTest(t)
	started := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{
		"repo":   makeIssueOpsCLIRepoForTest(t, "mcp-compatibility"),
		"branch": "125-compatibility-review",
	})
	id := started["id"].(string)
	callMCPToolForIssueOpsTest(t, "issueops_record_intent", map[string]any{
		"id":                 id,
		"raw_request":        "refactor issueops flow",
		"interpreted_intent": "keep compatibility and side-effect review before implementation",
		"success_criteria":   []any{"compatibility is reviewed"},
	})
	callMCPToolForIssueOpsTest(t, "issueops_link_issue", map[string]any{"id": id, "issue_url": "https://github.com/example/repo/issues/125"})
	callMCPToolForIssueOpsTest(t, "issueops_prepare_branch", map[string]any{
		"id": id, "provider": "github", "issue_url": "https://github.com/example/repo/issues/125", "branch": "125-compatibility-review", "base_branch": "main", "link_verified": true,
	})
	worktree := makeIssueOpsCLIWorktreeForTest(t, started["repo"].(string), "125-compatibility-review")
	callMCPToolForIssueOpsTest(t, "issueops_link_worktree", map[string]any{"id": id, "worktree_path": worktree})
	callMCPToolForIssueOpsTest(t, "issueops_review_design", map[string]any{
		"id": id, "problem_summary": "IssueOps must preserve the work contract", "proposed_design": "Gate implementation on compatibility review", "refactor_plan": "Keep changes scoped to IssueOps state", "alternatives": []any{"docs-only"}, "risks": []any{"golden drift"}, "verification": []any{"design review checked alternatives and risks"}, "approved": true,
	})
	writeMCPCompatibilityFileForTest(t, worktree, "plans/demo.md", "plan\n")
	callMCPToolForIssueOpsTest(t, "issueops_link_plan", map[string]any{"id": id, "plan_path": filepath.Join(worktree, "plans/demo.md")})

	payload := callMCPToolForIssueOpsTest(t, "issueops_record_compatibility_review", map[string]any{
		"id":                     id,
		"backward_compatibility": []any{"existing IssueOps JSON records remain readable"},
		"side_effects":           []any{"phase ordering changes are limited to IssueOps lifecycle gates"},
		"rollback_plan":          "revert the compatibility-review phase and readiness gate",
		"verification":           []any{"compatibility review checked backward compatibility and side effects"},
		"approved":               true,
	})
	review, ok := payload["compatibility_review"].(map[string]any)
	if !ok || review["approved"] != true {
		t.Fatalf("MCP compatibility review not persisted: %#v", payload)
	}
	if payload["phase"] != "compatibility-review" {
		t.Fatalf("compatibility review should move cycle to compatibility-review phase: %#v", payload)
	}
}

func writeMCPCompatibilityFileForTest(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
