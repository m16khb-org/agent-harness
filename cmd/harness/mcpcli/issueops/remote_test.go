package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestMCPIssueOpsSetPhaseAcceptsToAlias(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	callMCPToolForIssueOpsTest(t, "issueops_record_intent", map[string]any{
		"id":                 id,
		"raw_request":        "exercise the phase alias",
		"interpreted_intent": "advance the cycle to grill via the to alias",
		"success_criteria":   []any{"phase aliases to grill"},
	})
	record := callMCPToolForIssueOpsTest(t, "issueops_set_phase", map[string]any{"id": id, "to": "grill"})
	if record["phase"] != "grill" {
		t.Fatalf("expected MCP to alias to phase, got %#v", record)
	}
}

func TestMCPIssueOpsVerifyRemoteArtifactRejectsBeforePR(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	verifyErr := callMCPToolForIssueOpsTestError(t, "issueops_verify_remote_artifact", map[string]any{
		"id":        id,
		"provider":  "github",
		"kind":      "pr",
		"url":       "https://github.com/example/repo/pull/1",
		"labels":    []string{"bug"},
		"assignees": []string{"habin"},
	})
	if !strings.Contains(verifyErr, "remote artifact") || !strings.Contains(verifyErr, "before pr phase") {
		t.Fatalf("expected MCP remote artifact verification to reject before PR phase, got %q", verifyErr)
	}
}

func TestMCPIssueOpsCleanupStatusReportsMissingEvidence(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	status := callMCPToolForIssueOpsTest(t, "issueops_cleanup_status", map[string]any{"id": id})
	if status["ready"] == true {
		t.Fatalf("cleanup status must not be ready without merge/worktree evidence: %#v", status)
	}
	missing, ok := status["missing"].([]any)
	if !ok || len(missing) == 0 {
		t.Fatalf("cleanup status should explain missing evidence: %#v", status)
	}
	choices, ok := status["choices"].([]any)
	if !ok || len(choices) != 3 {
		t.Fatalf("cleanup status should expose three cleanup choices: %#v", status)
	}
}

func TestMCPIssueOpsCleanupCloseChildrenRecordsState(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	bin := t.TempDir()
	writeFakeGhForMCPCreateChild(t, bin)
	t.Setenv("PATH", bin)
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": repo, "branch": "12-child"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	callMCPToolForIssueOpsTest(t, "issueops_link_issue", map[string]any{
		"id":        id,
		"issue_url": "https://github.com/acme/repo/issues/12",
	})
	callMCPToolForIssueOpsTest(t, "issueops_prepare_branch", map[string]any{
		"id":            id,
		"provider":      "github",
		"issue_url":     "https://github.com/acme/repo/issues/12",
		"branch":        "12-child",
		"base_branch":   "main",
		"link_verified": true,
	})
	callMCPToolForIssueOpsTest(t, "issueops_remote_create_child", map[string]any{
		"id":        id,
		"title":     "Child",
		"body":      "Body",
		"labels":    []string{"bug"},
		"assignees": []string{"octocat"},
		"confirm":   true,
	})
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	record.RemoteArtifact = &core.IssueOpsRemoteArtifactVerification{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/acme/repo/pull/55",
		Labels:    []string{"issueops"},
		Assignees: []string{"octocat"},
	}
	if _, err := core.WriteIssueOps(core.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	missingMerge := callMCPToolForIssueOpsTestError(t, "issueops_cleanup_close_children", map[string]any{"id": id})
	if !strings.Contains(missingMerge, "merge evidence") {
		t.Fatalf("expected merge evidence failure, got %q", missingMerge)
	}

	closeResult := callMCPToolForIssueOpsTest(t, "issueops_cleanup_close_children", map[string]any{
		"id":      id,
		"merged":  true,
		"confirm": true,
	})
	if closeResult["closed_count"] != float64(1) || closeResult["dry_run"] == true {
		t.Fatalf("unexpected close-children result: %#v", closeResult)
	}
	status := callMCPToolForIssueOpsTest(t, "issueops_status", map[string]any{"id": id})
	links, ok := status["issue_links"].([]any)
	if !ok || len(links) != 1 {
		t.Fatalf("expected one child link in status: %#v", status)
	}
	link := links[0].(map[string]any)
	if link["close_verified_at"] == "" || link["close_reason"] != "completed" {
		t.Fatalf("expected verified child close evidence in state: %#v", link)
	}
}

func TestMCPIssueOpsPrepareWorktreeToolsPersistsEvidence(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())

	repo := makeIssueOpsCLIRepoForTest(t, "mcp-prepare")
	worktree := makeIssueOpsCLIWorktreeForTest(t, repo, "1-demo")
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": repo, "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	callMCPToolForIssueOpsTest(t, "issueops_link_issue", map[string]any{
		"id":        id,
		"issue_url": "https://github.com/example/repo/issues/1",
	})
	callMCPToolForIssueOpsTest(t, "issueops_prepare_branch", map[string]any{
		"id":            id,
		"provider":      "github",
		"issue_url":     "https://github.com/example/repo/issues/1",
		"branch":        "1-demo",
		"base_branch":   "main",
		"link_verified": true,
	})
	callMCPToolForIssueOpsTest(t, "issueops_link_worktree", map[string]any{
		"id":            id,
		"worktree_path": worktree,
	})

	prepared := callMCPToolForIssueOpsTest(t, "issueops_prepare_worktree_tools", map[string]any{"id": id})
	if prepared["ok"] != true || prepared["worktree_path"] != worktree {
		t.Fatalf("unexpected MCP prepare-tools result: %#v", prepared)
	}
}

func TestMCPIssueOpsRemoteScoreAcceptsCandidateAliases(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	result := callMCPToolForIssueOpsTest(t, "issueops_remote_score", map[string]any{
		"provider": "github",
		"issue": map[string]any{
			"title": "IssueOps feedback gate",
			"body":  "Feedback contract gate should block PR readiness.",
		},
		"related_issues": []map[string]any{{
			"id":    "#11",
			"title": "IssueOps feedback gate",
			"score": 0.93,
		}},
		"labels": []map[string]any{{
			"name":  "bug",
			"score": 0.91,
		}},
	})
	issues, ok := result["selected_related_issues"].([]any)
	if !ok || len(issues) != 1 {
		t.Fatalf("expected alias related issue to be selected: %#v", result)
	}
	labels, ok := result["selected_labels"].([]any)
	if !ok || len(labels) != 1 {
		t.Fatalf("expected alias label to be selected: %#v", result)
	}
}

func TestMCPIssueOpsRemoteCreateChildRecordsChildLink(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	bin := t.TempDir()
	writeFakeGhForMCPCreateChild(t, bin)
	t.Setenv("PATH", bin)
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": repo, "branch": "12-child"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	callMCPToolForIssueOpsTest(t, "issueops_link_issue", map[string]any{
		"id":        id,
		"issue_url": "https://github.com/acme/repo/issues/12",
	})
	callMCPToolForIssueOpsTest(t, "issueops_prepare_branch", map[string]any{
		"id":            id,
		"provider":      "github",
		"issue_url":     "https://github.com/acme/repo/issues/12",
		"branch":        "12-child",
		"base_branch":   "main",
		"link_verified": true,
	})

	result := callMCPToolForIssueOpsTest(t, "issueops_remote_create_child", map[string]any{
		"id":        id,
		"title":     "Child",
		"body":      "Body",
		"labels":    []string{"bug"},
		"assignees": []string{"octocat"},
		"confirm":   true,
	})
	if result["child_url"] != "https://github.com/acme/repo/issues/34" || result["hierarchy_verified"] != true {
		t.Fatalf("unexpected create-child result: %#v", result)
	}
	status := callMCPToolForIssueOpsTest(t, "issueops_status", map[string]any{"id": id})
	links, ok := status["issue_links"].([]any)
	if !ok || len(links) != 1 {
		t.Fatalf("expected one child link in status: %#v", status)
	}
}

func writeFakeGhForMCPCreateChild(t *testing.T, binDir string) {
	t.Helper()
	script := `#!/bin/sh
if [ "$1 $2" = "pr view" ]; then
  printf '{"url":"https://github.com/acme/repo/pull/55","state":"MERGED","mergedAt":"2026-06-17T00:00:00Z","labels":[{"name":"issueops"}],"assignees":[{"login":"octocat"}]}'
  exit 0
fi
if [ "$1 $2" = "issue create" ]; then
  printf 'https://github.com/acme/repo/issues/34\n'
  exit 0
fi
if [ "$1" = "api" ] && [ "$2" = "repos/acme/repo/issues/34" ]; then
  printf '{"id":987,"number":34,"html_url":"https://github.com/acme/repo/issues/34","state":"closed","labels":[{"name":"bug"}],"assignees":[{"login":"octocat"}]}'
  exit 0
fi
if [ "$1 $2" = "api -X" ] && [ "$3" = "POST" ]; then
  printf '{"ok":true}'
  exit 0
fi
if [ "$1 $2" = "api -X" ] && [ "$3" = "PATCH" ]; then
  printf '{"id":987,"number":34,"html_url":"https://github.com/acme/repo/issues/34","state":"closed"}'
  exit 0
fi
if [ "$1" = "api" ] && [ "$2" = "repos/acme/repo/issues/12/sub_issues" ]; then
  printf '[{"id":987,"number":34,"html_url":"https://github.com/acme/repo/issues/34"}]'
  exit 0
fi
echo "unexpected gh call: $*" >&2
exit 2
`
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestMCPIssueOpsRemoteCreateIssueConfirmVerifiesLiveIssue(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	bin := t.TempDir()
	writeFakeGhForMCPCreateIssue(t, bin, `[{"name":"bug"}]`)
	t.Setenv("PATH", bin)
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": repo, "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	result := callMCPToolForIssueOpsTest(t, "issueops_remote_create_issue", map[string]any{
		"id":        id,
		"provider":  "github",
		"title":     "Title",
		"body":      "Body",
		"labels":    []string{"bug"},
		"assignees": []string{"octocat"},
		"confirm":   true,
	})
	if result["url"] != "https://github.com/acme/repo/issues/77" {
		t.Fatalf("unexpected create-issue result: %#v", result)
	}
}

func TestMCPIssueOpsRemoteCreateIssueConfirmFailsWhenLiveLabelsMissing(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	bin := t.TempDir()
	// Live issue carries the wrong label, so the post-create verification gate
	// must reject the otherwise-successful creation.
	writeFakeGhForMCPCreateIssue(t, bin, `[{"name":"other"}]`)
	t.Setenv("PATH", bin)
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": repo, "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	createErr := callMCPToolForIssueOpsTestError(t, "issueops_remote_create_issue", map[string]any{
		"id":        id,
		"provider":  "github",
		"title":     "Title",
		"body":      "Body",
		"labels":    []string{"bug"},
		"assignees": []string{"octocat"},
		"confirm":   true,
	})
	if !strings.Contains(createErr, "label") {
		t.Fatalf("expected live label verification failure, got %q", createErr)
	}
}

func writeFakeGhForMCPCreateIssue(t *testing.T, binDir, labelsJSON string) {
	t.Helper()
	script := `#!/bin/sh
if [ "$1 $2" = "issue create" ]; then
  printf 'https://github.com/acme/repo/issues/77\n'
  exit 0
fi
if [ "$1 $2" = "issue view" ]; then
  printf '{"url":"https://github.com/acme/repo/issues/77","labels":` + labelsJSON + `,"assignees":[{"login":"octocat"}],"state":"OPEN"}'
  exit 0
fi
echo "unexpected gh call: $*" >&2
exit 2
`
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
