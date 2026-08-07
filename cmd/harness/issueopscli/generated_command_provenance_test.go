package issueopscli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/adapter/core"
	issueopscore "agent-harness/internal/adapter/issueops"
	commandparsecontract "agent-harness/internal/contract/commandparse"
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/domain/commandparse"
	provenanceport "agent-harness/internal/port/issueopsprovenance"
)

type issueOpsProvenanceObserverStub struct {
	evidence provenanceport.Receipt
}

func (s issueOpsProvenanceObserverStub) Observe(context.Context) (provenanceport.Receipt, error) {
	return s.evidence, nil
}

func TestGeneratedCommandRejectsStaleInstalledBinaryBeforeMutation(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "generated-command-provenance")
	record, err := issueopscore.StartIssueOps(core.IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "303-provenance"})
	if err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", "303-provenance")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	record.Execution = &issueopscontract.Execution{
		Mode:      issueopscontract.ExecutionModeDirect,
		Workspace: issueopscontract.Workspace{SourceRoot: repo, Root: worktree, Branch: record.Branch, BaseHead: "base", Driver: "git", LinkedAt: "2026-08-04T00:00:00Z"},
		Lease: issueopscontract.WriteLease{
			Generation: 7, Status: issueopscontract.LeaseStatusActive, ClaimedAt: "2026-08-04T00:00:00Z",
			Holder: &issueopscontract.NativeActor{Host: "codex", SessionID: "session-1", SessionProcess: &issueopscontract.NativeProcessReceipt{
				PID: 42, StartedAt: "2026-08-04T00:00:00Z", Executable: "/bin/codex",
			}},
		},
	}
	if _, err := issueopscore.WriteIssueOps(core.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	mutations := 0
	out, runErr := captureStdoutAndErrorForIssueOps(t, func() error {
		return RunIssueOpsWithDependencies([]string{
			"execution", "release", "--id", record.ID, "--generation", "7",
			"--generated-by-executable", "/worktree/bin/agent-harness",
			"--generated-by-sha256", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"--generated-for-generation", "7", "--json",
		}, Dependencies{
			Release: func(context.Context, string, issueopscore.ExecutionReleaseRequest) (issueopscore.ExecutionResult, error) {
				mutations++
				return issueopscore.ExecutionResult{}, nil
			},
			Provenance: issueOpsProvenanceObserverStub{evidence: provenanceport.Receipt{
				ExecutablePath:   "/installed/bin/agent-harness",
				ExecutableSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			}},
		})
	})
	if runErr == nil || mutations != 0 {
		t.Fatalf("stale binary run err=%v mutations=%d", runErr, mutations)
	}
	var failure map[string]any
	if err := json.Unmarshal([]byte(out), &failure); err != nil {
		t.Fatalf("structured mismatch output: %v\n%s", err, out)
	}
	if failure["code"] != "generated_command_binary_provenance_mismatch" || failure["lease_generation"] != float64(7) {
		t.Fatalf("mismatch payload = %#v", failure)
	}
}

func TestGeneratedCommandRunsExactObservedBinaryEnvelopeWithoutCallerRepair(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "generated-command-exact-binary")
	record, err := issueopscore.StartIssueOps(core.IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "303-exact-binary"})
	if err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", "303-exact-binary")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	record.Execution = &issueopscontract.Execution{
		Mode:      issueopscontract.ExecutionModeDirect,
		Workspace: issueopscontract.Workspace{SourceRoot: repo, Root: worktree, Branch: record.Branch, BaseHead: "base", Driver: "git", LinkedAt: "2026-08-04T00:00:00Z"},
		Lease: issueopscontract.WriteLease{
			Generation: 7, Status: issueopscontract.LeaseStatusActive, ClaimedAt: "2026-08-04T00:00:00Z",
			Holder: &issueopscontract.NativeActor{Host: "codex", SessionID: "session-1", SessionProcess: &issueopscontract.NativeProcessReceipt{
				PID: 42, StartedAt: "2026-08-04T00:00:00Z", Executable: "/bin/codex",
			}},
		},
	}
	if _, err := issueopscore.WriteIssueOps(core.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	evidence := commandparsecontract.GeneratedCommandProvenance{
		ExecutablePath: "/worktree/bin/agent-harness", ExecutableSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", LeaseGeneration: 7,
	}
	command, err := commandparsecontract.BindGeneratedCommand(
		"agent-harness issueops execution release --id "+record.ID+" --generation 7 --json",
		evidence,
	)
	if err != nil {
		t.Fatal(err)
	}
	tokens := commandparse.SplitCommandTokens(command)
	if len(tokens) < 3 || tokens[0] != evidence.ExecutablePath || tokens[1] != "issueops" {
		t.Fatalf("generated command does not select exact observed binary: %q", command)
	}
	mutations := 0
	_, runErr := captureStdoutAndErrorForIssueOps(t, func() error {
		return RunIssueOpsWithDependencies(tokens[2:], Dependencies{
			Release: func(context.Context, string, issueopscore.ExecutionReleaseRequest) (issueopscore.ExecutionResult, error) {
				mutations++
				return issueopscore.ExecutionResult{OK: true, ID: record.ID, Execution: *record.Execution}, nil
			},
			Provenance: issueOpsProvenanceObserverStub{evidence: provenanceport.Receipt{
				ExecutablePath: evidence.ExecutablePath, ExecutableSHA256: evidence.ExecutableSHA256,
			}},
		})
	})
	if runErr != nil || mutations != 1 {
		t.Fatalf("exact generated command err=%v mutations=%d", runErr, mutations)
	}
}

func TestGeneratedDelegatedChildBootstrapUsesParentExecutionProvenance(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "generated-delegated-bootstrap")
	parent, _ := startIssueOpsCLIReadyDelegationParent(t, repo, "123-334-parent")
	child := parent
	child.ID = "io-generated-delegated-child"
	child.Branch = "123-334-child-bootstrap"
	child.WorktreePath = ""
	child.Execution = nil
	child.Delegation = &issueopscontract.IssueOpsDelegationContract{
		ParentCycleID: parent.ID, TaskScope: "bootstrap", DelegatedAt: "2026-08-04T00:00:00Z",
	}
	parent.ChildCycles = append(parent.ChildCycles, issueopscontract.IssueOpsChildCycleRef{
		CycleID: child.ID, Branch: child.Branch, CreatedAt: "2026-08-04T00:00:00Z",
	})
	if _, err := issueopscore.WriteIssueOps(core.IssueOpsStateRoot(), parent); err != nil {
		t.Fatal(err)
	}
	if _, err := issueopscore.WriteIssueOps(core.IssueOpsStateRoot(), child); err != nil {
		t.Fatal(err)
	}
	evidence := commandparsecontract.GeneratedCommandProvenance{
		ExecutablePath: "/worktree/bin/agent-harness", ExecutableSHA256: strings.Repeat("a", 64), LeaseGeneration: 1,
	}
	args := []string{
		"branch", "prepare", "--id", child.ID, "--provider", "github",
		"--issue-url", "https://github.com/acme/repo/issues/334", "--branch", child.Branch,
		"--base-branch", parent.Branch, "--base-sha", strings.Repeat("b", 40),
		"--parent-worktree", parent.WorktreePath, "--link-verified",
		"--host", "codex", "--session-id", "session", "--cwd", parent.WorktreePath, "--json",
		"--generated-by-executable", evidence.ExecutablePath,
		"--generated-by-sha256", evidence.ExecutableSHA256,
		"--generated-for-generation", "1",
	}
	clean, generated, err := prepareGeneratedCommandInvocation(args, Dependencies{
		Provenance: issueOpsProvenanceObserverStub{evidence: provenanceport.Receipt{
			ExecutablePath: evidence.ExecutablePath, ExecutableSHA256: evidence.ExecutableSHA256,
		}},
	})
	if err != nil || !generated || len(clean) == 0 {
		t.Fatalf("delegated child bootstrap provenance was not resolved through parent: generated=%v clean=%v err=%v", generated, clean, err)
	}
}

func TestGeneratedOwnerMutationRequiresActualProcessCWD(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := makeIssueOpsCLIRepoForTest(t, "generated-owner-process-cwd")
	parent, actor := startIssueOpsCLIReadyDelegationParent(t, repo, "123-330-generated-owner-cwd")
	evidence := commandparsecontract.GeneratedCommandProvenance{
		ExecutablePath: "/worktree/bin/agent-harness", ExecutableSHA256: strings.Repeat("a", 64), LeaseGeneration: 1,
	}
	args := withIssueOpsCLIActor([]string{
		"child", "start", "--parent", parent.ID, "--branch", "330-generated-child",
		"--title", "generated child", "--scope", "verify process cwd", "--acceptance", "cwd fence holds", "--json",
	}, actor)
	args = append(args,
		"--generated-by-executable", evidence.ExecutablePath,
		"--generated-by-sha256", evidence.ExecutableSHA256,
		"--generated-for-generation", "1",
	)
	deps := Dependencies{Provenance: issueOpsProvenanceObserverStub{evidence: provenanceport.Receipt{
		ExecutablePath: evidence.ExecutablePath, ExecutableSHA256: evidence.ExecutableSHA256,
	}}}

	t.Chdir(repo)
	_, runErr := captureStdoutAndErrorForIssueOps(t, func() error {
		return RunIssueOpsWithDependencies(args, deps)
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "actual process cwd") {
		t.Fatalf("generated owner mutation from source cwd must fail before mutation: %v", runErr)
	}
	status, err := core.IssueOpsChildStatus(core.IssueOpsStateRoot(), parent.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Children) != 0 {
		t.Fatalf("cwd mismatch mutated parent children: %#v", status.Children)
	}

	t.Chdir(parent.WorktreePath)
	if _, runErr := captureStdoutAndErrorForIssueOps(t, func() error {
		return RunIssueOpsWithDependencies(args, deps)
	}); runErr != nil {
		t.Fatalf("matching actual process cwd must admit generated owner mutation: %v", runErr)
	}
}
