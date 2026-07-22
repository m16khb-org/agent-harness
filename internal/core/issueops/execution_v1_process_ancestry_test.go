package issueops

import (
	"os"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func TestNativeProcessAncestryFromSnapshotV1WalksExactParentChain(t *testing.T) {
	snapshot, err := parseNativeProcessSnapshotV1(`
100 50 Tue Jul 22 09:10:11 2026 /usr/local/bin/agent-harness
50 1 Tue Jul 22 09:00:00 2026 /Applications/Codex.app/Contents/MacOS/Codex
1 0 Tue Jul 22 08:00:00 2026 /sbin/launchd
`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := nativeProcessAncestryFromSnapshotV1(snapshot, 100)
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

func TestObserveNativeProcessAncestryV1IncludesCurrentExactReceipt(t *testing.T) {
	want, err := ObserveNativeProcessReceiptV1(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	got, err := ObserveNativeProcessAncestryV1(os.Getpid())
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

func TestNormalizeNativeActorV1RequiresReceiptInLocalProcessAncestry(t *testing.T) {
	receipt, err := ObserveNativeProcessReceiptV1(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	actor := model.NativeActorV1{
		Host: "codex", SessionID: "session", SessionProcess: &receipt,
		ProcessAncestry: []model.NativeProcessReceiptV1{receipt},
	}
	if _, err := normalizeNativeActorV1(actor); err != nil {
		t.Fatalf("exact locally observed process receipt rejected: %v", err)
	}

	actor.ProcessAncestry = nil
	if _, err := normalizeNativeActorV1(actor); err == nil {
		t.Fatal("payload receipt without local process ancestry was accepted")
	}
	actor.ProcessAncestry = []model.NativeProcessReceiptV1{{
		PID: receipt.PID, StartedAt: "1970-01-01T00:00:00Z", Executable: receipt.Executable,
	}}
	if _, err := normalizeNativeActorV1(actor); err == nil {
		t.Fatal("PID reuse mismatch in local process ancestry was accepted")
	}
}
