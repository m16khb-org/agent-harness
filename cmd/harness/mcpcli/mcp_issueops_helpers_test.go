package mcpcli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"agent-harness/cmd/harness/issueopscli/remotecmd"
	"agent-harness/internal/core"
	"agent-harness/internal/core/sqlstore"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCLIAndMCPRemoteCreateReconcileCallbacksCaptureIdenticalCompleteClaimProjection(t *testing.T) {
	record := core.IssueOpsRecord{
		Repo: "/workspace/repo",
		RemoteCreateClaim: &core.IssueOpsRemoteCreateClaim{
			Provider: "gitlab", Kind: "mr", ProjectKey: "gitlab.example:8443/group/nested/repo",
			Head: "issue/16", Base: "main", FinalHead: strings.Repeat("a", 40), Title: "canonical title", BodySHA256: strings.Repeat("b", 64),
			Labels: []string{"bug", "security"}, Assignees: []string{"alice", "bob"}, Draft: true,
		},
	}
	type capture struct {
		provider string
		request  core.IssueProviderReconcilePullRequestRequest
	}
	var cliCapture, mcpCapture capture
	cliProbe := remotecmd.RemoteCreateReconcileProbe(remotecmd.Deps{ReconcilePullRequest: func(providerName string, request core.IssueProviderReconcilePullRequestRequest) (core.IssueProviderReconcilePullRequestResult, error) {
		cliCapture = capture{provider: providerName, request: request}
		return core.IssueProviderReconcilePullRequestResult{AuthoritativeZero: true}, nil
	}})
	mcpProbe := remoteCreateReconcileProbe(func(providerName string, request core.IssueProviderReconcilePullRequestRequest) (core.IssueProviderReconcilePullRequestResult, error) {
		mcpCapture = capture{provider: providerName, request: request}
		return core.IssueProviderReconcilePullRequestResult{AuthoritativeZero: true}, nil
	})
	if _, err := cliProbe(context.Background(), record); err != nil {
		t.Fatalf("CLI reconcile callback: %v", err)
	}
	if _, err := mcpProbe(context.Background(), record); err != nil {
		t.Fatalf("MCP reconcile callback: %v", err)
	}
	want, err := core.ProjectIssueOpsRemoteCreateClaimForProviderReconcile(record)
	if err != nil {
		t.Fatal(err)
	}
	if cliCapture.provider != record.RemoteCreateClaim.Provider || mcpCapture.provider != record.RemoteCreateClaim.Provider || !reflect.DeepEqual(cliCapture, mcpCapture) || !reflect.DeepEqual(cliCapture.request, want) {
		t.Fatalf("adapter captures differ from complete durable claim projection: cli=%#v mcp=%#v want=%#v", cliCapture, mcpCapture, want)
	}
}

func TestIssueOpsMCPHelpersAndRemoteDryRuns(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := mcpIssueOpsRecord(t)

	if outcome := issueOpsMCPOutcome(map[string]any{"ok": true}, nil, "failed"); outcome.Err != nil || outcome.Payload == nil {
		t.Fatalf("success outcome = %#v", outcome)
	}
	if outcome := issueOpsMCPOutcome(nil, errMCPFixture("bad"), "failed"); outcome.Err != nil || !outcome.IsError {
		t.Fatalf("error outcome should be a normalized error result, got %#v", outcome)
	} else if body, ok := outcome.Payload.(map[string]any); !ok || body["ok"] != false || body["error"] != "failed: bad" {
		t.Fatalf("error outcome payload = %#v", outcome.Payload)
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
	if outcome := handleIssueOpsMCPToolCall(MCPToolCall{Name: "issueops_remote_score", Arguments: map[string]any{"bad": true}}); !outcome.IsError {
		t.Fatalf("invalid remote score call should be a normalized error result, got %#v", outcome)
	}
}

func TestMCPRemoteCreatePRLegacyAtMeParityAndNoStateMutation(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := mcpIssueOpsRecord(t)
	binDir := t.TempDir()
	script := filepath.Join(binDir, "gh")
	body := `#!/bin/sh
if [ "$1 $2" = "pr create" ]; then printf '%s\n' "$@" > gh.argv; printf 'https://github.com/acme/repo/pull/16\n'; exit 0; fi
if [ "$1 $2" = "pr view" ]; then printf '{"url":"https://github.com/acme/repo/pull/16","title":"PR","body":"legacy body","headRefName":"1234-mcp","baseRefName":"main","isDraft":false,"labels":[{"name":"bug"}],"assignees":[{"login":"octocat"}]}'; exit 0; fi
if [ "$1 $2" = "api user" ]; then printf 'octocat\n'; exit 0; fi
exit 2
`
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	db, err := sqlstore.Open(core.IssueOpsStateRoot())
	if err != nil {
		t.Fatal(err)
	}
	before, ok, err := db.Get("issueops", record.ID)
	if err != nil || !ok {
		t.Fatalf("read legacy bytes: ok=%v err=%v", ok, err)
	}
	arguments := map[string]any{
		"id": record.ID, "title": "PR", "body": "legacy body", "head": record.Branch, "base": "main",
		"labels": []any{"bug"}, "assignees": []any{"@me"}, "confirm": true,
	}
	rawArguments, err := json.Marshal(arguments)
	if err != nil {
		t.Fatal(err)
	}
	handler := sdkToolHandler(resolveHandlerGroup("issueops_remote_create_pr"), "issueops_remote_create_pr")
	result, err := handler(context.Background(), &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{Arguments: rawArguments}})
	if err != nil || result == nil || result.IsError || len(result.Content) != 1 {
		t.Fatalf("legacy MCP @me create result=%#v err=%v", result, err)
	}
	content, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("legacy MCP content type = %T", result.Content[0])
	}
	assertMCPGoldenBytes(t, "testdata/legacy_create_pr_content.golden.json", []byte(content.Text))
	encodedResult, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	assertMCPGoldenBytes(t, "testdata/legacy_create_pr_response.golden.json", encodedResult)
	argv, err := os.ReadFile(filepath.Join(record.Repo, "gh.argv"))
	if err != nil || !strings.Contains(string(argv), "\n--assignee\n@me\n") || strings.Contains(string(argv), "\n--draft\n") {
		t.Fatalf("legacy MCP provider argv = %q, err=%v", argv, err)
	}
	after, ok, err := db.Get("issueops", record.ID)
	if err != nil || !ok || !reflect.DeepEqual(after, before) {
		t.Fatalf("legacy MCP create mutated IssueOps bytes: ok=%v err=%v", ok, err)
	}
}

func assertMCPGoldenBytes(t *testing.T, path string, got []byte) {
	t.Helper()
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Repository text fixtures end with one newline; the MCP content itself does not.
	got = append(append([]byte(nil), got...), '\n')
	if !bytes.Equal(got, want) {
		t.Fatalf("MCP golden changed for %s\nwant=%q\n got=%q", path, want, got)
	}
}

func TestMCPRemoteCreatePRDryRunRedactsProviderArgvForGitHubAndGitLab(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := mcpIssueOpsRecord(t)
	secret := "api_key=opaque-token password=opaque-password Authorization: Bearer opaque-bearer /tmp/secret.pem"
	for _, providerName := range []string{"github", "gitlab"} {
		t.Run(providerName, func(t *testing.T) {
			outcome := handleMCPRemoteCreatePR(map[string]any{
				"id": record.ID, "provider": providerName, "title": "PR", "body": secret, "head": record.Branch, "base": "main",
			})
			if outcome.IsError || outcome.Err != nil {
				t.Fatalf("%s MCP dry-run = %#v", providerName, outcome)
			}
			result, ok := outcome.Payload.(core.IssueProviderCreatePullRequestResult)
			if !ok {
				t.Fatalf("%s MCP dry-run payload = %#v", providerName, outcome.Payload)
			}
			preview := result.Preview
			if strings.Contains(preview, "opaque-") || !strings.Contains(preview, "<redacted>") || len(preview) > 4300 {
				t.Fatalf("%s MCP dry-run preview was not bounded and redacted: %s", providerName, preview)
			}
		})
	}
}

// TestIssueOpsMCPToolCallReturnsNormalizedError pins the #8 contract: an IssueOps
// tool-level failure flows through HandleToolCall as a normalized error tool
// result ({ok:false,error:...} content marked isError) rather than a -32602
// "Invalid params" JSON-RPC protocol error. Before the fix HandleToolCall
// returned (nil, &RPCError{Code:-32602}); this asserts the new shape.
func TestIssueOpsMCPToolCallReturnsNormalizedError(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := mcpIssueOpsRecord(t)
	params, err := json.Marshal(map[string]any{
		"name":      "issueops_record_domain_review",
		"arguments": map[string]any{"id": record.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, rpcErr := HandleToolCall(params)
	if rpcErr != nil {
		t.Fatalf("tool-level failure must not be a JSON-RPC protocol error, got %#v", rpcErr)
	}
	m, ok := result.(map[string]any)
	if !ok || m["isError"] != true {
		t.Fatalf("expected isError tool result, got %#v", result)
	}
	content, ok := m["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("expected text content, got %#v", m["content"])
	}
	text, _ := content[0]["text"].(string)
	var body map[string]any
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("error body is not JSON: %v (%q)", err, text)
	}
	if body["ok"] != false {
		t.Fatalf("error body should report ok:false, got %#v", body)
	}
	if msg, _ := body["error"].(string); !strings.Contains(msg, "domain review requires model_fit or terminology") {
		t.Fatalf("error body missing core error: %#v", body)
	}
}

func TestIssueOpsMCPRemoteRenderTemplateAndCreateUsesTemplate(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := mcpIssueOpsRecord(t)

	rendered := handleIssueOpsMCPToolCall(MCPToolCall{Name: "issueops_remote_render_template", Arguments: map[string]any{
		"provider": "github",
		"kind":     "issue",
		"template": "feature",
		"title":    "원격 템플릿 계약",
		"fields": map[string]any{
			"problem":              "본문 품질이 흔들린다.",
			"current_evidence":     "임의 body만 받는다.",
			"acceptance_criteria":  "렌더러 결과가 고정된다.",
			"non_goals":            "provider 정책 복제 제외",
			"implementation_scope": "core와 MCP",
			"verification":         "go test ./...",
			"risks":                "golden drift",
			"feedback_log":         "없음",
		},
		"score_summary": "선택 라벨: enhancement, 거절 라벨: docs, threshold 0.70",
	}})
	if rendered.Err != nil {
		t.Fatalf("render-template MCP error: %#v", rendered.Err)
	}
	result, ok := rendered.Payload.(core.IssueOpsTemplateResult)
	if !ok || !strings.Contains(result.Body, "## 관련 이슈/라벨 판단") {
		t.Fatalf("unexpected render payload: %#v", rendered.Payload)
	}

	created := handleMCPRemoteCreateIssue(map[string]any{
		"id":       record.ID,
		"title":    "원격 템플릿 계약",
		"template": "feature",
		"labels":   []any{"bug"},
		"fields": map[string]any{
			"problem":              "본문 품질이 흔들린다.",
			"current_evidence":     "임의 body만 받는다.",
			"acceptance_criteria":  "렌더러 결과가 고정된다.",
			"non_goals":            "provider 정책 복제 제외",
			"implementation_scope": "core와 MCP",
			"verification":         "go test ./...",
			"risks":                "golden drift",
			"feedback_log":         "없음",
		},
	})
	if created.Err != nil {
		t.Fatalf("template create dry-run should render body before provider preview: %#v", created.Err)
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
			if !outcome.IsError {
				t.Fatalf("missing record prepare branch should be a normalized error result, got %#v", outcome)
			}
			continue
		}
		if outcome.Err != nil {
			t.Fatalf("%s outcome error: %#v", call.Name, outcome.Err)
		}
	}
	if outcome := handleIssueOpsMCPToolCall(MCPToolCall{Name: "issueops_link_child", Arguments: map[string]any{"id": record.ID, "child_url": "not-a-url"}}); !outcome.IsError {
		t.Fatalf("invalid child URL should be a normalized error result, got %#v", outcome)
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
