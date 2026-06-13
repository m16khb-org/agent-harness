package mcpcli

import (
	"encoding/json"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestIssueOpsMCPHelpersAndRemoteDryRuns(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := mcpIssueOpsRecord(t)

	if outcome := issueOpsMCPOutcome(map[string]any{"ok": true}, nil, "failed"); outcome.Err != nil || outcome.Payload == nil {
		t.Fatalf("success outcome = %#v", outcome)
	}
	if outcome := issueOpsMCPOutcome(nil, errMCPFixture("bad"), "failed"); outcome.Err == nil || outcome.Err.Code != -32602 {
		t.Fatalf("error outcome = %#v", outcome)
	}
	if provider := resolveRecordProviderForMCP(core.IssueOpsRecord{BranchPrepare: &core.IssueOpsBranchPrepare{Provider: "github"}}); provider != "github" {
		t.Fatalf("branch provider = %q", provider)
	}
	if provider := resolveRecordProviderForMCP(core.IssueOpsRecord{RemoteArtifact: &core.IssueOpsRemoteArtifactVerification{Provider: "gitlab"}}); provider != "gitlab" {
		t.Fatalf("artifact provider = %q", provider)
	}
	if provider := resolveRecordProviderForMCP(core.IssueOpsRecord{IssueURL: "https://github.com/acme/repo/issues/1"}); provider != "github" {
		t.Fatalf("issue URL provider = %q", provider)
	}
	if _, err := issueOpsRemoteScoringRequestFromMCP(map[string]any{"bad": make(chan int)}); err == nil {
		t.Fatal("non-marshalable scoring request should fail")
	}
	if _, err := verifyIssueOpsRemoteArtifactFromMCP(map[string]any{"id": record.ID, "provider": "github", "kind": "ticket", "url": "not-a-url"}); err == nil {
		t.Fatal("invalid remote artifact verification should fail before live verification")
	}
	req, err := issueOpsRemoteScoringRequestFromMCP(map[string]any{
		"provider":         "github",
		"threshold":        0.5,
		"issue":            map[string]any{"title": "Fix", "body": "Body"},
		"issue_candidates": []any{map[string]any{"id": "1", "title": "Fix", "score": 0.9}},
	})
	if err != nil || req.Provider != "github" {
		t.Fatalf("scoring request = %#v err=%v", req, err)
	}
	for name, args := range map[string]map[string]any{
		"issue": {"id": record.ID, "title": "Child", "body": "Body", "labels": []any{"bug"}},
		"pr":    {"id": record.ID, "title": "PR", "body": "Body"},
		"sync":  {"id": record.ID},
	} {
		var outcome MCPToolOutcome
		switch name {
		case "issue":
			outcome = handleMCPRemoteCreateIssue(args)
		case "pr":
			outcome = handleMCPRemoteCreatePR(args)
		case "sync":
			outcome = handleMCPRemoteSyncGraph(args)
		}
		if outcome.Err != nil {
			t.Fatalf("%s dry-run error: %#v", name, outcome.Err)
		}
	}
	if outcome := handleIssueOpsMCPToolCall(MCPToolCall{Name: "issueops_remote_score", Arguments: map[string]any{"bad": true}}); outcome.Err == nil {
		t.Fatal("invalid remote score call should fail")
	}
}

func TestTransportExportedWrappers(t *testing.T) {
	if len(MCPResources()) == 0 {
		t.Fatal("MCPResources should expose resources")
	}
	out := captureStatusVerifyStdout(t, func() error {
		WriteRPCResult(json.RawMessage(`1`), map[string]any{"ok": true})
		WriteRPCError(json.RawMessage(`2`), -1, "bad", "data")
		return nil
	})
	if !strings.Contains(out, `"result"`) || !strings.Contains(out, `"error"`) {
		t.Fatalf("RPC wrapper output missing result/error: %s", out)
	}
}

func TestHandleIssueOpsMCPToolCallLifecycleCases(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := handleIssueOpsMCPToolCall(MCPToolCall{Name: "issueops_start", Arguments: map[string]any{
		"repo":   t.TempDir(),
		"branch": "1235-mcp-flow",
	}})
	if start.Err != nil {
		t.Fatalf("start outcome: %#v", start.Err)
	}
	record, ok := start.Payload.(core.IssueOpsRecord)
	if !ok || record.ID == "" {
		t.Fatalf("start payload = %#v", start.Payload)
	}
	cases := []MCPToolCall{
		{Name: "issueops_status", Arguments: map[string]any{"id": record.ID}},
		{Name: "issueops_link_issue", Arguments: map[string]any{"id": record.ID, "issue_url": "https://github.com/acme/repo/issues/1235"}},
		{Name: "issueops_record_intent", Arguments: map[string]any{
			"id": record.ID, "raw_request": "raw", "interpreted_intent": "intent", "success_criteria": []any{"pass"},
		}},
		{Name: "issueops_review_design", Arguments: map[string]any{
			"id": record.ID, "problem_summary": "problem", "proposed_design": "design", "refactor_plan": "plan", "alternatives": []any{"alt"}, "risks": []any{"risk"}, "verification": []any{"go test", "design review checked alternatives and risks"}, "approved": true,
		}},
		{Name: "issueops_prepare_branch", Arguments: map[string]any{
			"id": "io-missing", "provider": "github", "issue_url": "https://github.com/acme/repo/issues/1235", "branch": "1235-mcp-flow", "base_branch": "main", "link_verified": true,
		}},
		{Name: "issueops_add_feedback", Arguments: map[string]any{"id": record.ID, "source": "review", "body": "fix", "classification": "defect"}},
		{Name: "issueops_add_decision", Arguments: map[string]any{"id": record.ID, "title": "Decision", "body": "Body", "kind": "architecture"}},
		{Name: "issueops_pr_readiness", Arguments: map[string]any{"id": record.ID}},
		{Name: "issueops_pr_readiness", Arguments: map[string]any{"id": record.ID, "strict": true}},
		{Name: "issueops_cleanup_status", Arguments: map[string]any{"id": record.ID, "merged": false}},
		{Name: "issueops_force_release", Arguments: map[string]any{"id": record.ID, "reason": "covered in lifecycle test"}},
		{Name: "issueops_cleanup_stale", Arguments: map[string]any{"repo": t.TempDir(), "max_age": 1}},
		{Name: "issueops_resume", Arguments: map[string]any{"repo": t.TempDir()}},
	}
	for _, call := range cases {
		outcome := handleIssueOpsMCPToolCall(call)
		if call.Name == "issueops_prepare_branch" {
			if outcome.Err == nil {
				t.Fatal("missing record prepare branch should fail")
			}
			continue
		}
		if outcome.Err != nil {
			t.Fatalf("%s outcome error: %#v", call.Name, outcome.Err)
		}
	}
	if outcome := handleIssueOpsMCPToolCall(MCPToolCall{Name: "issueops_link_child", Arguments: map[string]any{"id": record.ID, "child_url": "not-a-url"}}); outcome.Err == nil {
		t.Fatal("invalid child URL should fail")
	}
	if outcome := handleIssueOpsMCPToolCall(MCPToolCall{Name: "unknown"}); outcome.Handled {
		t.Fatalf("unknown IssueOps tool should not be handled: %#v", outcome)
	}
}

func mcpIssueOpsRecord(t *testing.T) core.IssueOpsRecord {
	t.Helper()
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "1234-mcp"})
	if err != nil {
		t.Fatalf("StartIssueOps: %v", err)
	}
	record, err = core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), record.ID, "https://github.com/acme/repo/issues/1234")
	if err != nil {
		t.Fatalf("LinkIssueOpsIssue: %v", err)
	}
	record, err = core.PrepareIssueOpsBranch(core.IssueOpsStateRoot(), record.ID, core.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     "https://github.com/acme/repo/issues/1234",
		Branch:       record.Branch,
		BaseBranch:   "main",
		LinkVerified: true,
	})
	if err != nil {
		t.Fatalf("PrepareIssueOpsBranch: %v", err)
	}
	return record
}

type errMCPFixture string

func (e errMCPFixture) Error() string { return string(e) }
