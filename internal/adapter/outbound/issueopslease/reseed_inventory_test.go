package issueopslease

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
	"agent-harness/internal/port"
)

func TestReseedInventoryFingerprintIncludesRawOwnerEvidence(t *testing.T) {
	root := t.TempDir()
	reseedInventoryGit(t, root, "init", "--initial-branch", "holderless-reseed")
	reseedInventoryGit(t, root, "config", "user.email", "test@example.invalid")
	reseedInventoryGit(t, root, "config", "user.name", "Test")
	reseedInventoryGit(t, root, "commit", "--allow-empty", "--message", "initial")
	head := strings.TrimSpace(reseedInventoryGit(t, root, "rev-parse", "HEAD"))
	record := leasecontract.Record{
		ID: "io-reseed-inventory",
		Execution: &leasecontract.Execution{
			Mode: "orca",
			Workspace: leasecontract.Workspace{
				SourceRoot: root, Root: root, Branch: "holderless-reseed", BaseHead: head, Driver: "orca", LinkedAt: "2026-08-03T00:00:00Z",
			},
			Lease: leasecontract.Lease{Generation: 3, Status: "claimable", ClaimTokenSHA256: strings.Repeat("a", 64)},
			Orca:  &leasecontract.OrcaBinding{RuntimeID: "runtime-old", WorktreeID: "worktree", RunID: "run", TaskID: "task", DispatchID: "dispatch", TerminalPTYID: "pty-old"},
		},
	}
	owner := &reseedInventoryOwnerStub{inventory: port.ExecutionOrcaOwnerInventory{
		RuntimeID: "runtime-current", TerminalInventoryComplete: true, TaskStatus: "failed", DispatchStatus: "dispatched", DispatchAssigneeHandle: "term-old",
	}}
	inventory := NewReseedInventory(owner, func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
		return "none", leasedomain.ProcessReceipt{}, nil
	})
	actor := leasedomain.Actor{Host: "codex", SessionID: "session"}
	baseline, err := inventory.Observe(context.Background(), record, actor)
	if err != nil {
		t.Fatal(err)
	}
	if baseline.Inventory.DispatchStatus != "dispatched" || !baseline.Inventory.TerminalInventoryComplete || baseline.Fingerprint == "" {
		t.Fatalf("owner evidence was not preserved in the reseed receipt: %#v", baseline)
	}

	owner.inventory.DispatchStatus = "pending"
	pending, err := inventory.Observe(context.Background(), record, actor)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Fingerprint == baseline.Fingerprint {
		t.Fatal("dispatch status change did not invalidate the reseed fingerprint")
	}

	owner.inventory.DispatchStatus = "dispatched"
	owner.inventory.TerminalInventoryComplete = false
	incomplete, err := inventory.Observe(context.Background(), record, actor)
	if err != nil {
		t.Fatal(err)
	}
	if incomplete.Fingerprint == baseline.Fingerprint {
		t.Fatal("terminal inventory completeness change did not invalidate the reseed fingerprint")
	}
}

type reseedInventoryOwnerStub struct {
	inventory port.ExecutionOrcaOwnerInventory
}

func (s *reseedInventoryOwnerStub) InspectOwner(context.Context, port.ExecutionOrcaOwnerInventoryRequest) (port.ExecutionOrcaOwnerInventory, error) {
	return s.inventory, nil
}

func reseedInventoryGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
