package issueopslease

import (
	"context"
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
	"agent-harness/internal/port"
)

func TestReseedWriteFingerprintFileRejectsChangedUntrackedFile(t *testing.T) {
	tests := []struct {
		name   string
		change func(t *testing.T, path string)
	}{
		{
			name: "replaced",
			change: func(t *testing.T, path string) {
				replacement := path + ".replacement"
				if err := os.WriteFile(replacement, []byte("same"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(replacement, path); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "resized",
			change: func(t *testing.T, path string) {
				if err := os.WriteFile(path, []byte("different-size"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "untracked")
			if err := os.WriteFile(path, []byte("same"), 0o600); err != nil {
				t.Fatal(err)
			}
			entry, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			test.change(t, path)

			err = reseedWriteFingerprintFile(sha256.New(), path, entry)
			if err == nil || !strings.Contains(err.Error(), "untracked file changed while snapshotting") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestReseedWorkspaceSnapshotStreamsLargeUntrackedFiles(t *testing.T) {
	root := t.TempDir()
	reseedInventoryGit(t, root, "init", "--initial-branch", "reseed")
	reseedInventoryGit(t, root, "config", "user.email", "test@example.invalid")
	reseedInventoryGit(t, root, "config", "user.name", "Test")
	reseedInventoryGit(t, root, "commit", "--allow-empty", "--message", "initial")
	large := filepath.Join(root, "large.bin")
	file, err := os.Create(large)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(32 << 20); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	if _, err := reseedWorkspaceSnapshot(leasecontract.Workspace{Root: root, Branch: "reseed"}); err != nil {
		t.Fatal(err)
	}
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 8<<20 {
		t.Fatalf("reseed snapshot allocated %d bytes while hashing a 32 MiB untracked file", allocated)
	}
}

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
