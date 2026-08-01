package issueopscli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"agent-harness/cmd/harness/mcpcli"
	"agent-harness/internal/core"
	issueopscore "agent-harness/internal/core/issueops"
	"agent-harness/internal/core/preflight"
)

func TestIssueOpsExecutionDepsPropagatePublicationReconcileWithoutInvocation(t *testing.T) {
	invoked := 0
	handler := issueopscore.RemotePullRequestReconcileHandler(func(context.Context, string, issueopscore.ExecutionReconcileRequest) (issueopscore.ExecutionReconcileResult, error) {
		invoked++
		return issueopscore.ExecutionReconcileResult{}, nil
	})

	deps := issueOpsExecutionDeps(Dependencies{Publication: issueopscore.RemotePublicationHandlers{Reconcile: handler}})
	if deps.Publication.Reconcile == nil {
		t.Fatal("publication reconcile handler was not propagated")
	}
	if reflect.ValueOf(deps.Publication.Reconcile).Pointer() != reflect.ValueOf(handler).Pointer() {
		t.Fatal("publication reconcile handler changed during CLI composition")
	}
	if invoked != 0 {
		t.Fatalf("publication reconcile handler invoked during propagation: %d", invoked)
	}
}

func TestIssueOpsExecutionCLIPrepareAndStatusShareSchemaProjection(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo, id, actorFlags := executionCLIRecord(t)

	preparedJSON := captureStdoutForContract(t, func() error {
		return runIssueOps(append([]string{
			"execution", "prepare", "--id", id, "--mode", "direct", "--cwd", repo, "--confirm", "--json",
		}, actorFlags...))
	})
	var prepared issueopscore.ExecutionPrepareResult
	if err := json.Unmarshal([]byte(preparedJSON), &prepared); err != nil {
		t.Fatalf("execution prepare should return JSON: %v\n%s", err, preparedJSON)
	}
	if !prepared.OK || prepared.Execution == nil || prepared.ResolvedMode != "direct" || prepared.Execution.Lease.Generation != 1 {
		t.Fatalf("unexpected prepare projection: %#v", prepared)
	}
	if prepared.Workspace.Root == repo || !strings.HasPrefix(prepared.Workspace.Root, repo+".worktrees"+string(filepath.Separator)) {
		t.Fatalf("prepare did not select the canonical sibling worktree: %#v", prepared.Workspace)
	}
	if top := preflight.GitOut(prepared.Workspace.Root, "rev-parse", "--show-toplevel"); !sameExecutionCLIPath(top, prepared.Workspace.Root) {
		t.Fatalf("prepare did not create a real linked worktree: top=%q", top)
	}

	statusJSON := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"execution", "status", "--id", id, "--json"})
	})
	var status issueopscore.ExecutionResult
	if err := json.Unmarshal([]byte(statusJSON), &status); err != nil {
		t.Fatalf("execution status should return JSON: %v\n%s", err, statusJSON)
	}
	if !reflect.DeepEqual(status.Execution, *prepared.Execution) {
		t.Fatalf("CLI status and prepare must expose the same execution DTO\nprepare=%#v\nstatus=%#v", prepared.Execution, status.Execution)
	}
}

func sameExecutionCLIPath(left, right string) bool {
	left, leftErr := filepath.EvalSymlinks(left)
	right, rightErr := filepath.EvalSymlinks(right)
	return leftErr == nil && rightErr == nil && filepath.Clean(left) == filepath.Clean(right)
}

func TestIssueOpsExecutionCLIRejectsLegacyDecideAndAmbiguousReplace(t *testing.T) {
	if err := runIssueOps([]string{"execution", "decide"}); err == nil || !strings.Contains(err.Error(), "unknown issueops execution subcommand") {
		t.Fatalf("legacy execution decide must be absent, got %v", err)
	}

	_, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"execution", "replace", "--id", "io-test", "--expected-generation", "1", "--preview", "--revoke", "--json"})
	})
	if err == nil || !strings.Contains(err.Error(), "exactly one action") {
		t.Fatalf("replace must require exactly one explicit action, got %v", err)
	}
}

func TestIssueOpsExecutionCLIAndMCPStatusAndErrorsAreIdentical(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo, id, actorFlags := executionCLIRecord(t)
	_ = captureStdoutForContract(t, func() error {
		return runIssueOps(append([]string{"execution", "prepare", "--id", id, "--mode", "direct", "--cwd", repo, "--confirm", "--json"}, actorFlags...))
	})

	cliJSON := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"execution", "status", "--id", id, "--json"})
	})
	mcpJSON := executionMCPText(t, map[string]any{"action": "status", "id": id})
	var cliResult, mcpResult issueopscore.ExecutionResult
	if err := json.Unmarshal([]byte(cliJSON), &cliResult); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(mcpJSON), &mcpResult); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cliResult, mcpResult) {
		t.Fatalf("CLI and MCP execution DTOs diverged\nCLI=%#v\nMCP=%#v", cliResult, mcpResult)
	}

	cliErrorJSON, cliErr := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOps([]string{"execution", "status", "--id", "io-missing", "--json"})
	})
	if cliErr == nil {
		t.Fatal("missing CLI execution must fail")
	}
	mcpErrorJSON := executionMCPText(t, map[string]any{"action": "status", "id": "io-missing"})
	var cliFailure, mcpFailure map[string]any
	if json.Unmarshal([]byte(cliErrorJSON), &cliFailure) != nil || json.Unmarshal([]byte(mcpErrorJSON), &mcpFailure) != nil {
		t.Fatalf("CLI/MCP failures must be JSON: cli=%s mcp=%s", cliErrorJSON, mcpErrorJSON)
	}
	if !reflect.DeepEqual(cliFailure, mcpFailure) {
		t.Fatalf("CLI and MCP error contracts diverged: cli=%#v mcp=%#v", cliFailure, mcpFailure)
	}
}

func executionMCPText(t *testing.T, arguments map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"name": "issueops_execution", "arguments": arguments})
	if err != nil {
		t.Fatal(err)
	}
	result, rpcErr := mcpcli.HandleToolCall(raw)
	if rpcErr != nil {
		t.Fatalf("MCP execution call failed at protocol layer: %#v", rpcErr)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(encoded, &envelope); err != nil || len(envelope.Content) != 1 {
		t.Fatalf("unexpected MCP result envelope: err=%v result=%s", err, encoded)
	}
	return envelope.Content[0].Text
}

func executionCLIRecord(t *testing.T) (string, string, []string) {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "issueops@example.invalid"},
		{"config", "user.name", "IssueOps Test"},
	} {
		if code, _, stderr := preflight.GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v: %s", args, stderr)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := preflight.GitCmd(repo, "add", "README.md"); code != 0 {
		t.Fatalf("git add: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "commit", "-q", "-m", "fixture"); code != 0 {
		t.Fatalf("git commit: %s", stderr)
	}
	baseHead := preflight.GitOut(repo, "rev-parse", "HEAD")
	branch := "69-execution-cli"
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/example/agent-harness/issues/69"
	record.BranchPrepare = &core.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: branch,
		BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true,
	}
	if _, err := core.WriteIssueOps(core.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	receipt, err := issueopscore.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	flags := []string{
		"--host", "codex", "--session-id", "cli-session", "--session-pid", fmt.Sprint(receipt.PID),
		"--session-started-at", receipt.StartedAt, "--session-executable", receipt.Executable,
	}
	return repo, record.ID, flags
}
