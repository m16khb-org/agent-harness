package mcpcli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPIssueOpsSetPhaseAcceptsToAlias(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
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
	rpcErr := callMCPToolForIssueOpsTestError(t, "issueops_verify_remote_artifact", map[string]any{
		"id":        id,
		"provider":  "github",
		"kind":      "pr",
		"url":       "https://github.com/example/repo/pull/1",
		"labels":    []string{"bug"},
		"assignees": []string{"habin"},
	})
	if rpcErr == nil || !strings.Contains(rpcErr.Message, "remote artifact") || !strings.Contains(fmt.Sprint(rpcErr.Data), "before pr phase") {
		t.Fatalf("expected MCP remote artifact verification to reject before PR phase, got %+v", rpcErr)
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

func TestMCPIssueOpsPrepareWorktreeToolsRunsCodeGraphAgainstWorktree(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "codegraph.log")
	codegraph := filepath.Join(bin, "codegraph")
	if err := os.WriteFile(codegraph, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> '"+logPath+"'\ncase \"$1\" in\nstatus) exit 1 ;;\ninit) exit 0 ;;\n*) exit 0 ;;\nesac\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

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
	if prepared["codegraph_ready"] != true || prepared["codegraph_project_path"] != worktree {
		t.Fatalf("unexpected MCP prepare-tools result: %#v", prepared)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "status "+worktree) || !strings.Contains(string(log), "init -i "+worktree) {
		t.Fatalf("codegraph should be checked and initialized against worktree, got:\n%s", log)
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
