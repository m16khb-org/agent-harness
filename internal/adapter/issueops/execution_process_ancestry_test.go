package issueops

import (
	"os"
	"testing"

	"agent-harness/internal/contract/issueops"
)

func TestNativeProcessAncestryFromSnapshotWalksExactParentChain(t *testing.T) {
	snapshot, err := parseNativeProcessSnapshot(`
100 50 Tue Jul 22 09:10:11 2026 /usr/local/bin/agent-harness
50 1 Tue Jul 22 09:00:00 2026 /Applications/Codex.app/Contents/MacOS/Codex
1 0 Tue Jul 22 08:00:00 2026 /sbin/launchd
`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := nativeProcessAncestryFromSnapshot(snapshot, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("ancestry length = %d, want 3: %+v", len(got), got)
	}
	if got[0].PID != 100 || got[0].Executable != "/usr/local/bin/agent-harness" || got[0].StartedAt == "" {
		t.Fatalf("child receipt = %+v", got[0])
	}
	if got[1].PID != 50 || got[1].Executable != "/Applications/Codex.app/Contents/MacOS/Codex" {
		t.Fatalf("parent receipt = %+v", got[1])
	}
	if got[2].PID != 1 || got[2].Executable != "/sbin/launchd" {
		t.Fatalf("root receipt = %+v", got[2])
	}
}

func TestQuiescenceProcessOwnershipReusesOneSnapshot(t *testing.T) {
	snapshot, err := parseNativeProcessSnapshot(`
200 100 Tue Jul 22 09:11:00 2026 /usr/bin/child
100 50 Tue Jul 22 09:10:11 2026 /usr/local/bin/agent-harness
50 1 Tue Jul 22 09:00:00 2026 /Applications/Codex.app/Contents/MacOS/Codex
300 1 Tue Jul 22 09:12:00 2026 /usr/bin/external
1 0 Tue Jul 22 08:00:00 2026 /sbin/launchd
`)
	if err != nil {
		t.Fatal(err)
	}
	ancestry := nativeProcessAncestryPIDsFromSnapshot(snapshot, 100)
	if !ancestry[100] || !ancestry[50] || !ancestry[1] {
		t.Fatalf("requester ancestry = %+v", ancestry)
	}
	processes := []workspaceProcess{
		{PID: 200, Command: "child"},
		{PID: 300, Command: "external"},
	}

	got := dropRequesterOwnedProcessesFromSnapshot(
		processes,
		map[int]bool{100: true},
		snapshot,
	)

	if len(got) != 1 || got[0].PID != 300 {
		t.Fatalf("remaining processes = %+v", got)
	}
}

func TestObserveNativeProcessAncestryIncludesCurrentExactReceipt(t *testing.T) {
	want, err := ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	got, err := ObserveNativeProcessAncestry(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	for _, receipt := range got {
		if receipt == want {
			return
		}
	}
	t.Fatalf("current process receipt %+v not found in ancestry %+v", want, got)
}

func TestNormalizeNativeActorRequiresReceiptInLocalProcessAncestry(t *testing.T) {
	receipt, err := ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	actor := issueops.NativeActor{
		Host: "codex", SessionID: "session", SessionProcess: &receipt,
		ProcessAncestry: []issueops.NativeProcessReceipt{receipt},
	}
	if _, err := normalizeNativeActor(actor); err != nil {
		t.Fatalf("exact locally observed process receipt rejected: %v", err)
	}

	actor.ProcessAncestry = nil
	if _, err := normalizeNativeActor(actor); err == nil {
		t.Fatal("payload receipt without local process ancestry was accepted")
	}
	actor.ProcessAncestry = []issueops.NativeProcessReceipt{{
		PID: receipt.PID, StartedAt: "1970-01-01T00:00:00Z", Executable: receipt.Executable,
	}}
	if _, err := normalizeNativeActor(actor); err == nil {
		t.Fatal("PID reuse mismatch in local process ancestry was accepted")
	}
}
