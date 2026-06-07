package issueops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/cmd/harness/issueopscli"
	"agent-harness/cmd/harness/mcpcli"
	"agent-harness/internal/core"
)

func callMCPToolForIssueOpsTest(t *testing.T, name string, args map[string]any) map[string]any {
	t.Helper()
	params, err := json.Marshal(map[string]any{"name": name, "arguments": args})
	if err != nil {
		t.Fatal(err)
	}
	result, rpcErr := mcpcli.HandleToolCall(params)
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

func callMCPToolForIssueOpsTestError(t *testing.T, name string, args map[string]any) *mcpcli.RPCError {
	t.Helper()
	params, err := json.Marshal(map[string]any{"name": name, "arguments": args})
	if err != nil {
		t.Fatal(err)
	}
	_, rpcErr := mcpcli.HandleToolCall(params)
	return rpcErr
}

func configureIssueOpsMCPForTest(t *testing.T) {
	t.Helper()
	previousPrepare := mcpcli.PrepareIssueOpsWorktreeTools
	previousVerifyChild := mcpcli.VerifyIssueOpsChildIssueBeforeLink
	previousCleanupMerged := mcpcli.IssueOpsCleanupMerged
	previousVerifyRemote := mcpcli.VerifyIssueOpsRemoteArtifactLive
	mcpcli.PrepareIssueOpsWorktreeTools = func(record core.IssueOpsRecord) (any, error) {
		return issueopscli.PrepareWorktreeTools(record)
	}
	mcpcli.VerifyIssueOpsChildIssueBeforeLink = issueopscli.VerifyChildIssueBeforeLink
	mcpcli.IssueOpsCleanupMerged = issueopscli.CleanupMerged
	mcpcli.VerifyIssueOpsRemoteArtifactLive = issueopscli.VerifyRemoteArtifactLive
	t.Cleanup(func() {
		mcpcli.PrepareIssueOpsWorktreeTools = previousPrepare
		mcpcli.VerifyIssueOpsChildIssueBeforeLink = previousVerifyChild
		mcpcli.IssueOpsCleanupMerged = previousCleanupMerged
		mcpcli.VerifyIssueOpsRemoteArtifactLive = previousVerifyRemote
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
