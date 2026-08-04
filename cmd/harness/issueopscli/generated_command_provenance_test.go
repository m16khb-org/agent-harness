package issueopscli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/core"
	"agent-harness/internal/core/commandparse"
	issueopscore "agent-harness/internal/core/issueops"
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
	evidence := issueopscontract.GeneratedCommandProvenance{
		ExecutablePath: "/worktree/bin/agent-harness", ExecutableSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", LeaseGeneration: 7,
	}
	command, err := issueopscontract.BindGeneratedCommand(
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
