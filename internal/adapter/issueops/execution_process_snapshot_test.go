package issueops

import (
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestInspectNativeProcessReceiptUsesCapturedSnapshot(t *testing.T) {
	receipt := issueopscontract.NativeProcessReceipt{
		PID:        42,
		StartedAt:  "2026-08-14T00:00:00Z",
		Executable: "/usr/bin/agent",
	}
	snapshot := map[int]nativeProcessSnapshotEntry{
		42: {Receipt: receipt},
	}

	status, observed, err := inspectNativeProcessReceiptFromSnapshot(receipt, snapshot)

	if err != nil || status != NativeProcessStatusLive || observed != receipt {
		t.Fatalf("status=%s observed=%+v error=%v", status, observed, err)
	}
	snapshot[42] = nativeProcessSnapshotEntry{Receipt: issueopscontract.NativeProcessReceipt{
		PID:        42,
		StartedAt:  receipt.StartedAt,
		Executable: "/usr/bin/reused",
	}}
	status, observed, err = inspectNativeProcessReceiptFromSnapshot(receipt, snapshot)
	if err != nil || status != NativeProcessStatusIdentityMismatch || observed.Executable != "/usr/bin/reused" {
		t.Fatalf("mismatch status=%s observed=%+v error=%v", status, observed, err)
	}
}
