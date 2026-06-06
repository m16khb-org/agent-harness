package mcpcli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/cmd/harness/issueopscli"
	"agent-harness/internal/core"
)

func TestMCPIssueOpsStartAndStatus(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" || start["phase"] != "problem" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	status := callMCPToolForIssueOpsTest(t, "issueops_status", map[string]any{"id": id})
	if status["id"] != id || status["repo"] != "/repo/example" {
		t.Fatalf("unexpected MCP status payload: %#v", status)
	}
}

func TestMCPIssueOpsRecordsIntentAndDesignReview(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	intent := callMCPToolForIssueOpsTest(t, "issueops_record_intent", map[string]any{
		"id":                 id,
		"raw_request":        "IssueOps must understand intent",
		"interpreted_intent": "Persist main-agent intent before planning",
		"success_criteria":   []string{"intent is recorded"},
		"constraints":        []string{"keep state durable"},
		"ambiguities":        []string{"none"},
		"non_goals":          []string{"do not implement from hook recommendation alone"},
	})
	if _, ok := intent["intent"].(map[string]any); !ok {
		t.Fatalf("MCP intent record should persist intent payload: %#v", intent)
	}
	callMCPToolForIssueOpsTest(t, "issueops_link_issue", map[string]any{
		"id":        id,
		"issue_url": "https://github.com/example/repo/issues/1",
	})
	design := callMCPToolForIssueOpsTest(t, "issueops_review_design", map[string]any{
		"id":              id,
		"problem_summary": "IssueOps needs a design gate",
		"proposed_design": "Require approved design before implementation",
		"refactor_plan":   "Keep changes in IssueOps core and adapters",
		"alternatives":    []string{"docs-only guidance"},
		"risks":           []string{"legacy tests need explicit setup"},
		"verification":    []string{"go test ./cmd/harness/mcpcli"},
		"approved":        true,
	})
	if review, ok := design["design_review"].(map[string]any); !ok || review["approved"] != true {
		t.Fatalf("MCP design review should persist approval payload: %#v", design)
	}
}

func TestMCPIssueOpsIntentAndDesignRejectInvalidInputs(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	intentErr := callMCPToolForIssueOpsTestError(t, "issueops_record_intent", map[string]any{
		"id":                 id,
		"raw_request":        "IssueOps must understand intent",
		"interpreted_intent": "Persist main-agent intent before planning",
	})
	if intentErr == nil || !strings.Contains(fmt.Sprint(intentErr.Data), "success_criteria is required") {
		t.Fatalf("expected MCP intent validation error, got %+v", intentErr)
	}
	missingVerificationErr := callMCPToolForIssueOpsTestError(t, "issueops_review_design", map[string]any{
		"id":              id,
		"problem_summary": "IssueOps needs a design gate",
		"proposed_design": "Require approved design before implementation",
	})
	if missingVerificationErr == nil || !strings.Contains(fmt.Sprint(missingVerificationErr.Data), "verification is required") {
		t.Fatalf("expected MCP design missing verification error, got %+v", missingVerificationErr)
	}
	designErr := callMCPToolForIssueOpsTestError(t, "issueops_review_design", map[string]any{
		"id":              id,
		"problem_summary": "IssueOps needs a design gate",
		"proposed_design": "Require approved design before implementation",
		"verification":    []string{"go test ./cmd/harness/mcpcli"},
		"open_questions":  []string{"which design?"},
		"approved":        true,
	})
	if designErr == nil || !strings.Contains(fmt.Sprint(designErr.Data), "open_questions") {
		t.Fatalf("expected MCP design validation error, got %+v", designErr)
	}
}

func TestMCPIssueOpsSetPhasePinsIntentAndIssuePlanGate(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	intentErr := callMCPToolForIssueOpsTestError(t, "issueops_set_phase", map[string]any{"id": id, "to": "plan"})
	if intentErr == nil || !strings.Contains(fmt.Sprint(intentErr.Data), "intent_contract") {
		t.Fatalf("expected MCP plan phase to require intent, got %+v", intentErr)
	}
	callMCPToolForIssueOpsTest(t, "issueops_record_intent", map[string]any{
		"id":                 id,
		"raw_request":        "IssueOps must understand intent",
		"interpreted_intent": "Persist main-agent intent before planning",
		"success_criteria":   []string{"intent is recorded"},
	})
	issueErr := callMCPToolForIssueOpsTestError(t, "issueops_set_phase", map[string]any{"id": id, "to": "plan"})
	if issueErr == nil || !strings.Contains(fmt.Sprint(issueErr.Data), "issue_url") {
		t.Fatalf("expected MCP plan phase to require linked issue after intent, got %+v", issueErr)
	}
}

func TestMCPIssueOpsIntentRedactsSecretLikeFreeform(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	intent := callMCPToolForIssueOpsTest(t, "issueops_record_intent", map[string]any{
		"id":                 id,
		"raw_request":        "token=secret-value",
		"interpreted_intent": "api_key=secret-value",
		"success_criteria":   []string{"password=secret-value"},
	})
	payload, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if strings.Contains(text, "secret-value") || (!strings.Contains(text, "<redacted>") && !strings.Contains(text, `\u003credacted\u003e`)) {
		t.Fatalf("MCP intent response should redact secret-like values:\n%s", text)
	}
}

func TestMCPIssueOpsLinkChild(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	stubIssueOpsChildIssueVerifierForMCPTest(t, nil)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	callMCPToolForIssueOpsTest(t, "issueops_link_issue", map[string]any{
		"id":        id,
		"issue_url": "https://gitlab.example/group/project/-/issues/1",
	})
	record := callMCPToolForIssueOpsTest(t, "issueops_link_child", map[string]any{
		"id":        id,
		"child_url": "https://gitlab.example/group/project/-/issues/2",
		"title":     "write GitLab child task",
	})
	links, ok := record["issue_links"].([]any)
	if !ok || len(links) != 1 {
		t.Fatalf("expected one issue link, got %#v", record)
	}
	link, ok := links[0].(map[string]any)
	if !ok || link["type"] != "child" || link["provider"] != "gitlab" {
		t.Fatalf("unexpected issue link: %#v", links[0])
	}
}

func TestMCPIssueOpsPrepareBranch(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "123-provider-linked-branch"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	callMCPToolForIssueOpsTest(t, "issueops_link_issue", map[string]any{
		"id":        id,
		"issue_url": "https://gitlab.example/group/project/-/issues/123",
	})
	record := callMCPToolForIssueOpsTest(t, "issueops_prepare_branch", map[string]any{
		"id":          id,
		"provider":    "gitlab",
		"issue_url":   "https://gitlab.example/group/project/-/issues/123",
		"branch":      "123-provider-linked-branch",
		"base_branch": "main",
	})
	prepare, ok := record["branch_prepare"].(map[string]any)
	if !ok || prepare["provider"] != "gitlab" || prepare["branch"] != "123-provider-linked-branch" {
		t.Fatalf("unexpected branch prepare payload: %#v", record)
	}
	steps, ok := prepare["steps"].([]any)
	if !ok || len(steps) != 3 {
		t.Fatalf("expected provider fallback steps: %#v", prepare)
	}
	first, ok := steps[0].(map[string]any)
	if !ok || first["strategy"] != "mcp" || first["tool"] != "mcp__glab.glab_api" {
		t.Fatalf("first branch prepare step must use GitLab MCP: %#v", steps[0])
	}
}

func TestMCPIssueOpsMarkIssueUpdated(t *testing.T) {
	configureIssueOpsMCPForTest(t)
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	start := callMCPToolForIssueOpsTest(t, "issueops_start", map[string]any{"repo": "/repo/example", "branch": "1-demo"})
	id, ok := start["id"].(string)
	if !ok || id == "" {
		t.Fatalf("unexpected MCP start payload: %#v", start)
	}
	callMCPToolForIssueOpsTest(t, "issueops_add_feedback", map[string]any{
		"id":             id,
		"source":         "review",
		"body":           "acceptance criteria changed",
		"classification": "contract_change",
	})
	record := callMCPToolForIssueOpsTest(t, "issueops_mark_issue_updated", map[string]any{"id": id})
	feedback, ok := record["feedback"].([]any)
	if !ok || len(feedback) != 1 {
		t.Fatalf("expected one feedback item, got %#v", record)
	}
	item, ok := feedback[0].(map[string]any)
	if !ok || item["issue_updated_at"] == "" {
		t.Fatalf("expected issue_updated_at after MCP mark, got %#v", feedback[0])
	}
}

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

func callMCPToolForIssueOpsTest(t *testing.T, name string, args map[string]any) map[string]any {
	t.Helper()
	params, err := json.Marshal(map[string]any{"name": name, "arguments": args})
	if err != nil {
		t.Fatal(err)
	}
	result, rpcErr := HandleToolCall(params)
	if rpcErr != nil {
		t.Fatalf("unexpected MCP rpc error: %+v", rpcErr)
	}
	outer, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected MCP result type %T", result)
	}
	content, ok := outer["content"].([]map[string]any)
	if !ok || len(content) != 1 {
		t.Fatalf("unexpected MCP content: %#v", outer["content"])
	}
	text, ok := content[0]["text"].(string)
	if !ok {
		t.Fatalf("unexpected MCP text content: %#v", content[0])
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("invalid MCP JSON text: %v\n%s", err, text)
	}
	return payload
}

func callMCPToolForIssueOpsTestError(t *testing.T, name string, args map[string]any) *RPCError {
	t.Helper()
	params, err := json.Marshal(map[string]any{"name": name, "arguments": args})
	if err != nil {
		t.Fatal(err)
	}
	_, rpcErr := HandleToolCall(params)
	return rpcErr
}

func configureIssueOpsMCPForTest(t *testing.T) {
	t.Helper()
	previousPrepare := PrepareIssueOpsWorktreeTools
	previousVerifyChild := VerifyIssueOpsChildIssueBeforeLink
	previousCleanupMerged := IssueOpsCleanupMerged
	previousVerifyRemote := VerifyIssueOpsRemoteArtifactLive
	PrepareIssueOpsWorktreeTools = func(record core.IssueOpsRecord) (any, error) {
		return issueopscli.PrepareWorktreeTools(record)
	}
	VerifyIssueOpsChildIssueBeforeLink = issueopscli.VerifyChildIssueBeforeLink
	IssueOpsCleanupMerged = issueopscli.CleanupMerged
	VerifyIssueOpsRemoteArtifactLive = issueopscli.VerifyRemoteArtifactLive
	t.Cleanup(func() {
		PrepareIssueOpsWorktreeTools = previousPrepare
		VerifyIssueOpsChildIssueBeforeLink = previousVerifyChild
		IssueOpsCleanupMerged = previousCleanupMerged
		VerifyIssueOpsRemoteArtifactLive = previousVerifyRemote
	})
}

func makeIssueOpsCLIRepoForTest(t *testing.T, name string) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	return repo
}

func makeIssueOpsCLIWorktreeForTest(t *testing.T, repo, slug string) string {
	t.Helper()
	worktree := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", slug)
	if err := os.MkdirAll(filepath.Join(worktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git", "HEAD"), []byte("ref: refs/heads/"+slug+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return worktree
}

func stubIssueOpsChildIssueVerifierForMCPTest(t *testing.T, verifier func(string) error) {
	t.Helper()
	previous := issueopscli.SetChildIssueVerifier(verifier)
	t.Cleanup(func() {
		issueopscli.SetChildIssueVerifier(previous)
	})
}
