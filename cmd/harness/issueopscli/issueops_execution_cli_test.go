package issueopscli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"agent-harness/cmd/harness/mcpcli"
	"agent-harness/internal/adapter/core"
	issueopscore "agent-harness/internal/adapter/issueops"
	commandparsecontract "agent-harness/internal/contract/commandparse"
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/domain/commandparse"
	provenanceport "agent-harness/internal/port/issueopsprovenance"
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

func TestIssueOpsExecutionDepsPropagateCompletionWithoutInvocation(t *testing.T) {
	invoked := 0
	handler := issueopscore.ExecutionCompleteHandler(func(context.Context, string, issueopscore.ExecutionCompleteRequest) (issueopscore.ExecutionResult, error) {
		invoked++
		return issueopscore.ExecutionResult{}, nil
	})
	deps := issueOpsExecutionDeps(Dependencies{Complete: handler})
	if deps.Complete == nil || reflect.ValueOf(deps.Complete).Pointer() != reflect.ValueOf(handler).Pointer() {
		t.Fatal("completion handler was not propagated unchanged")
	}
	if invoked != 0 {
		t.Fatalf("completion handler invoked during propagation: %d", invoked)
	}
}

func TestIssueOpsExecutionPrepareCLIAndStatusShareSchemaProjection(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo, id, actorFlags := executionCLIRecord(t)
	deps := Dependencies{
		Prepare: executionCLIPrepareHandler(t),
		Provenance: issueOpsProvenanceObserverStub{evidence: provenanceport.Receipt{
			ExecutablePath: "/repo/bin/agent-harness", ExecutableSHA256: strings.Repeat("e", 64),
		}},
	}

	preparedJSON := captureStdoutForContract(t, func() error {
		return runIssueOpsWithDependencies(append([]string{
			"execution", "prepare", "--id", id, "--mode", "direct", "--cwd", repo, "--confirm", "--json",
		}, actorFlags...), deps)
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

	statusJSON := captureStdoutForContract(t, func() error {
		return RunIssueOpsWithDependencies([]string{"execution", "status", "--id", id, "--json"}, deps)
	})
	var status issueopscore.ExecutionResult
	if err := json.Unmarshal([]byte(statusJSON), &status); err != nil {
		t.Fatalf("execution status should return JSON: %v\n%s", err, statusJSON)
	}
	if !reflect.DeepEqual(status.Execution, *prepared.Execution) {
		t.Fatalf("CLI status and prepare must expose the same execution DTO\nprepare=%#v\nstatus=%#v", prepared.Execution, status.Execution)
	}
}

func TestIssueOpsExecutionStatusProjectsActorFreeResumeCommand(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo, id, _ := executionCLIRecord(t)
	record, err := issueopscore.ReadIssueOps(core.IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(repo+".worktrees", record.Branch)
	record.WorktreePath = worktree
	record.Execution = &issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeOrca,
		Workspace: issueopscontract.Workspace{
			SourceRoot: repo, Root: worktree, Branch: record.Branch,
			BaseHead: record.BranchPrepare.BaseSHA, Driver: "orca", LinkedAt: "2026-08-02T00:00:00Z",
		},
		Lease: issueopscontract.WriteLease{
			Generation: 3, Status: issueopscontract.LeaseStatusClaimable,
			ClaimTokenSHA256: strings.Repeat("a", 64),
		},
		Orca: &issueopscontract.OrcaBinding{
			RuntimeID: "runtime-1", RepoID: "repo-1", WorktreeID: "worktree-1",
			LeaseGeneration: 2, OwnerHost: "codex", OwnerModel: "gpt-5.6-terra",
			ArtifactIdentityVersion: issueopscontract.OrcaArtifactIdentityVersion,
			IssueBodySHA256:         strings.Repeat("b", 64), ContextPacketSHA256: strings.Repeat("c", 64), OwnerPromptSHA256: strings.Repeat("d", 64),
			TaskID: "task-1", DispatchID: "dispatch-1", TerminalPTYID: "pty-1",
		},
	}
	if _, err := issueopscore.WriteIssueOps(core.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	deps := Dependencies{Provenance: issueOpsProvenanceObserverStub{evidence: provenanceport.Receipt{
		ExecutablePath: "/repo/bin/agent-harness", ExecutableSHA256: strings.Repeat("e", 64),
	}}}
	statusJSON := captureStdoutForContract(t, func() error {
		return RunIssueOpsWithDependencies([]string{"execution", "status", "--id", id, "--json"}, deps)
	})
	var status issueopscore.ExecutionResult
	if err := json.Unmarshal([]byte(statusJSON), &status); err != nil {
		t.Fatalf("execution status should return JSON: %v\n%s", err, statusJSON)
	}
	want := issueopscore.ExecutionResumeRecoveryCommand(id, 3)
	if !sameGeneratedExecutionCommand(status.NextCommand, want, 3) {
		t.Fatalf("status next command = %q, want %q", status.NextCommand, want)
	}

	record.Execution.Orca.IssueBodySHA256 = ""
	record.Execution.Orca.ContextPacketSHA256 = ""
	record.Execution.Orca.OwnerPromptSHA256 = ""
	record.Execution.Orca.ArtifactIdentityVersion = 0
	if _, err := issueopscore.WriteIssueOps(core.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	legacyJSON := captureStdoutForContract(t, func() error {
		return RunIssueOpsWithDependencies([]string{"execution", "status", "--id", id, "--json"}, deps)
	})
	if err := json.Unmarshal([]byte(legacyJSON), &status); err != nil {
		t.Fatalf("legacy execution status should return JSON: %v\n%s", err, legacyJSON)
	}
	want = "agent-harness issueops execution replace --id '" + id + "' --expected-generation 3 --preview"
	if !sameGeneratedExecutionCommand(status.NextCommand, want, 3) {
		t.Fatalf("legacy status next command = %q, want %q", status.NextCommand, want)
	}
}

func sameGeneratedExecutionCommand(got, raw string, generation uint64) bool {
	tokens := commandparse.SplitCommandTokens(got)
	if len(tokens) < 2 {
		return false
	}
	clean, provenance, present, err := commandparsecontract.ConsumeGeneratedCommandProvenance(tokens[1:])
	if err != nil || !present || provenance.LeaseGeneration != generation || tokens[0] != provenance.ExecutablePath {
		return false
	}
	want := commandparse.SplitCommandTokens(raw)
	return len(want) > 1 && strings.Join(clean, "\x00") == strings.Join(want[1:], "\x00")
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

func TestIssueOpsExecutionPrepareCLIAndMCPStatusAndErrorsAreIdentical(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo, id, actorFlags := executionCLIRecord(t)
	_ = captureStdoutForContract(t, func() error {
		return runIssueOpsWithDependencies(
			append([]string{"execution", "prepare", "--id", id, "--mode", "direct", "--cwd", repo, "--confirm", "--json"}, actorFlags...),
			Dependencies{Prepare: executionCLIPrepareHandler(t)},
		)
	})
	record, err := issueopscore.ReadIssueOps(core.IssueOpsStateRoot(), id)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution.Lease.Generation = 2
	record.Execution.CompletionHistory = []issueopscontract.ExecutionCompletionHistory{{
		Generation: 1,
		Completion: issueopscontract.ExecutionCompletion{Generation: 1, FinalHead: strings.Repeat("a", 40), TuringReportPath: ".agent-harness/turing/old.json", Verification: []string{"old verification"}, RemoteArtifactURL: "https://github.com/acme/repo/pull/304", CompletedAt: "2026-08-03T00:00:00Z"},
		Reason:     "functional HEAD changed",
		ReopenedAt: "2026-08-04T00:00:00Z",
	}}
	if _, err := issueopscore.WriteIssueOps(core.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

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
	if len(cliResult.Execution.CompletionHistory) != 1 || cliResult.Execution.CompletionHistory[0].Completion.Verification[0] != "old verification" {
		t.Fatalf("CLI/MCP status lost completion history: %+v", cliResult.Execution.CompletionHistory)
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

func TestIssueOpsExecutionPrepareCLIFailsClosedWithoutHandler(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo, id, actorFlags := executionCLIRecord(t)
	_, err := captureStdoutAndErrorForIssueOps(t, func() error {
		return runIssueOpsWithDependencies(
			append([]string{"execution", "prepare", "--id", id, "--mode", "direct", "--cwd", repo, "--json"}, actorFlags...),
			Dependencies{},
		)
	})
	if !errors.Is(err, issueopscore.ErrPrepareHandlerUnavailable) {
		t.Fatalf("prepare without handler error = %v", err)
	}
}

func executionCLIPrepareHandler(t *testing.T) issueopscore.ExecutionPrepareHandler {
	t.Helper()
	return func(_ context.Context, stateRoot string, request issueopscore.ExecutionPrepareRequest, _ issueopscore.ExecutionPrepareInvocation) (issueopscore.ExecutionPrepareResult, error) {
		record, err := issueopscore.ReadIssueOps(stateRoot, request.ID)
		if err != nil {
			return issueopscore.ExecutionPrepareResult{ID: request.ID}, err
		}
		actor := request.Actor
		actor.ProcessAncestry = nil
		workspace := issueopscontract.Workspace{
			SourceRoot: record.Repo,
			Root:       filepath.Join(record.Repo+".worktrees", record.Branch),
			Branch:     record.Branch,
			BaseHead:   record.BranchPrepare.BaseSHA,
			Driver:     "git",
			LinkedAt:   "2026-08-02T00:00:00Z",
		}
		execution := &issueopscontract.Execution{
			Mode:      issueopscontract.ExecutionModeDirect,
			Workspace: workspace,
			Lease: issueopscontract.WriteLease{
				Generation: 1,
				Status:     issueopscontract.LeaseStatusActive,
				Holder:     &actor,
				ClaimedAt:  workspace.LinkedAt,
			},
		}
		record.WorktreePath = workspace.Root
		record.Execution = execution
		written, err := issueopscore.WriteIssueOps(stateRoot, record)
		if err != nil {
			return issueopscore.ExecutionPrepareResult{ID: request.ID}, err
		}
		return issueopscore.ExecutionPrepareResult{
			OK: true, ID: request.ID, RequestedMode: request.Mode, ResolvedMode: "direct",
			Workspace: workspace, Execution: written.Execution,
		}, nil
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
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/example/agent-harness/issues/69"
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{
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
