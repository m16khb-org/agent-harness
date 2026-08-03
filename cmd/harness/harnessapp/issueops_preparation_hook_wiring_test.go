package harnessapp

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/cmd/harness/hookcli"
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/port"
	"agent-harness/internal/testsupport"
)

func TestExecutionPrepareEnabledHooksPreserveDirectHolderAuthority(t *testing.T) {
	for _, host := range []string{"codex", "claude"} {
		t.Run(host, func(t *testing.T) {
			_, record, actor := prepareHookExecutionFixture(t, "direct", host)
			root := record.Execution.Workspace.Root
			if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
				t.Fatal(err)
			}
			output := runEnabledPreparationHook(t, host, root, actor, filepath.Join(root, "owned.go"))
			if len(output) != 0 {
				t.Fatalf("exact holder canonical mutation must be allowed: %#v", output)
			}
			output = runEnabledPreparationHook(t, host, record.Repo, actor, filepath.Join(root, "sub", "owned.go"))
			if len(output) != 0 {
				t.Fatalf("explicit canonical target from source cwd must be allowed: %#v", output)
			}
			output = runEnabledPreparationHook(t, host, filepath.Join(root, "sub"), actor, filepath.Join(root, "sub", "nested.go"))
			if len(output) != 0 {
				t.Fatalf("canonical subdirectory mutation must be allowed: %#v", output)
			}

			alias := filepath.Join(root, "outside-alias")
			if err := os.Symlink(t.TempDir(), alias); err != nil {
				t.Fatal(err)
			}
			output = runEnabledPreparationHook(t, host, root, actor, filepath.Join(alias, "escaped.go"))
			assertPreparationHookDeny(t, host, output, record.ID, root, 1, "write_lease_required")
		})
	}
}

func TestExecutionPrepareEnabledHooksDenyForeignAndStaleHolderIdentity(t *testing.T) {
	for _, host := range []string{"codex", "claude"} {
		t.Run(host, func(t *testing.T) {
			stateRoot, prepared, actor := prepareHookExecutionFixture(t, "direct", host)
			root := prepared.Execution.Workspace.Root
			target := filepath.Join(root, "owned.go")
			foreign := actor
			foreign.SessionID = "foreign-session"
			output := runEnabledPreparationHook(t, host, root, foreign, target)
			assertPreparationHookDeny(t, host, output, prepared.ID, root, 1, "holder_identity_mismatch")

			for name, mutate := range map[string]func(*issueopscontract.NativeProcessReceipt){
				"pid start":  func(receipt *issueopscontract.NativeProcessReceipt) { receipt.StartedAt = "1970-01-01T00:00:00Z" },
				"executable": func(receipt *issueopscontract.NativeProcessReceipt) { receipt.Executable = "/foreign/codex" },
			} {
				t.Run(name, func(t *testing.T) {
					record, err := issueops.ReadIssueOps(stateRoot, prepared.ID)
					if err != nil {
						t.Fatal(err)
					}
					receipt := *actor.SessionProcess
					mutate(&receipt)
					holder := actor
					holder.ProcessAncestry = nil
					holder.SessionProcess = &receipt
					record.Execution.Lease.Holder = &holder
					if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
						t.Fatal(err)
					}
					output := runEnabledPreparationHook(t, host, root, actor, target)
					assertPreparationHookDeny(t, host, output, prepared.ID, root, 1, "holder_identity_mismatch")
				})
			}
		})
	}
}

func TestExecutionPrepareEnabledHooksExposeOrcaWriterlessRecovery(t *testing.T) {
	stateRoot, prepared, actor := prepareHookExecutionFixture(t, "orca", "codex")
	root := prepared.Execution.Workspace.Root
	target := filepath.Join(root, "owned.go")

	for _, test := range []struct {
		status issueopscontract.LeaseStatus
		code   string
	}{
		{status: issueopscontract.LeaseStatusClaimable, code: "lease_claimable"},
		{status: issueopscontract.LeaseStatusReleased, code: "lease_released"},
		{status: issueopscontract.LeaseStatusRevoking, code: "lease_revoking"},
	} {
		t.Run(string(test.status), func(t *testing.T) {
			record, err := issueops.ReadIssueOps(stateRoot, prepared.ID)
			if err != nil {
				t.Fatal(err)
			}
			record.Execution.Lease = issueopscontract.WriteLease{Generation: 1, Status: test.status}
			switch test.status {
			case issueopscontract.LeaseStatusClaimable:
				record.Execution.Lease.ClaimTokenSHA256 = strings.Repeat("a", 64)
			case issueopscontract.LeaseStatusRevoking:
				holder := actor
				holder.ProcessAncestry = nil
				record.Execution.Lease.Holder = &holder
			}
			if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}
			for _, host := range []string{"codex", "claude"} {
				output := runEnabledPreparationHook(t, host, root, actor, target)
				assertPreparationHookDeny(t, host, output, prepared.ID, root, 1, test.code)
			}
		})
	}
}

func prepareHookExecutionFixture(t *testing.T, mode, host string) (string, issueopscontract.IssueOpsRecord, issueopscontract.NativeActor) {
	t.Helper()
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	stateRoot := issueops.IssueOpsStateRoot()
	repo := filepath.Join(t.TempDir(), "source")
	claimWiringGit(t, "", "init", "-q", "-b", "main", repo)
	claimWiringGit(t, repo, "-c", "user.name=IssueOps Test", "-c", "user.email=issueops@example.invalid", "commit", "--allow-empty", "-q", "-m", "fixture")
	baseHead := strings.TrimSpace(claimWiringGit(t, repo, "rev-parse", "HEAD"))
	branch := "199-hook-" + mode + "-" + host
	record, err := issueops.StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/199"
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: branch,
		BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true,
	}
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	actor := claimWiringActor(t)
	actor.Host = host
	actor.SessionID = "prepare-hook-holder"
	actor.AgentID = "prepare-hook-agent"
	direct := &hookPreparationDirectFake{}
	orca := &reconcileProvisionerFake{}
	handler := newIssueOpsPreparationHandler(issueOpsPreparationCompositionDeps{
		Direct: direct, Orca: orca,
		ReadIssue: func(_ context.Context, _ string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
			return port.ExecutionIssueSnapshot{URL: request.URL, Body: claimWiringIssueBody()}, nil
		},
		NewOperationID: func() (string, error) { return "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", nil },
	})
	request := issueops.ExecutionPrepareRequest{
		ID: record.ID, Mode: mode, Actor: actor, CWD: repo,
		OwnerHost: host, OwnerModel: "test-model", OwnerEffort: "high",
	}
	if mode == "direct" {
		request.DirectReason = "hook authority test"
	}
	preview, err := handler(context.Background(), stateRoot, request, issueops.ExecutionPrepareInvocation{})
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedReadinessFingerprint, request.Confirm = preview.ReadinessFingerprint, true
	result, err := handler(context.Background(), stateRoot, request, issueops.ExecutionPrepareInvocation{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Execution == nil || result.ResolvedMode != mode {
		t.Fatalf("prepare result=%#v", result)
	}
	prepared, err := issueops.ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, prepared, actor
}

type hookPreparationDirectFake struct{}

func (*hookPreparationDirectFake) ProbeAccess(context.Context, port.ExecutionWorkspaceRequest, string) (port.ExecutionWorkspaceAccessResult, error) {
	return port.ExecutionWorkspaceAccessResult{Allowed: true}, nil
}

func (*hookPreparationDirectFake) Prepare(_ context.Context, request port.ExecutionWorkspaceRequest) (port.ExecutionWorkspaceReceipt, error) {
	if err := os.MkdirAll(request.Root, 0o755); err != nil {
		return port.ExecutionWorkspaceReceipt{}, err
	}
	return port.ExecutionWorkspaceReceipt{
		SourceRoot: request.SourceRoot, Root: request.Root, Branch: request.Branch,
		BaseHead: request.BaseHead, ParentWorktree: request.ParentWorktree, Driver: "git", Exists: true,
	}, nil
}

func runEnabledPreparationHook(t *testing.T, host, cwd string, actor issueopscontract.NativeActor, target string) map[string]any {
	t.Helper()
	tool := "apply_patch"
	if host == "claude" {
		tool = "Edit"
	}
	payload, err := json.Marshal(map[string]any{
		"cwd": cwd, "host": host, "session_id": actor.SessionID, "agent_id": actor.AgentID,
		"tool_name": tool, "tool_input": map[string]any{"file_path": target},
	})
	if err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = reader
	go func() {
		_, _ = io.WriteString(writer, string(payload))
		_ = writer.Close()
	}()
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = reader.Close()
	})
	output := testsupport.CaptureStdout(t, func() error {
		return hookcli.RunHookPreToolUse([]string{"--host", host, "--repo", cwd, "--enforce-worktree"})
	})
	os.Stdin = oldStdin
	_ = reader.Close()
	var decoded map[string]any
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("hook output=%q err=%v", output, err)
	}
	return decoded
}

func assertPreparationHookDeny(t *testing.T, host string, output map[string]any, id, root string, generation int, code string) {
	t.Helper()
	reason := ""
	if host == "claude" {
		specific, _ := output["hookSpecificOutput"].(map[string]any)
		if specific["hookEventName"] != "PreToolUse" || specific["permissionDecision"] != "deny" {
			t.Fatalf("Claude deny schema=%#v", output)
		}
		reason, _ = specific["permissionDecisionReason"].(string)
	} else {
		if output["decision"] != "block" {
			t.Fatalf("Codex deny schema=%#v", output)
		}
		reason, _ = output["reason"].(string)
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(reason), &fields); err != nil {
		t.Fatalf("structured deny reason=%q err=%v", reason, err)
	}
	if fields["code"] != code || fields["lifecycle_id"] != id || fields["expected_root"] != root ||
		fields["current_generation"] != float64(generation) || !strings.Contains(fields["next_command"].(string), "--id "+id) {
		t.Fatalf("structured deny fields=%#v", fields)
	}
}
