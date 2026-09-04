package issueopslease

import (
	"encoding/json"
	"testing"
)

func TestReseedReceiptJSONRoundTripPreservesV1Execution(t *testing.T) {
	want := ReseedReceipt{
		Execution:      Execution{Mode: "direct", Workspace: Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "feature", BaseHead: "abc", Driver: "git", LinkedAt: "2026-07-30T08:00:00Z"}, Lease: Lease{Generation: 2, Status: "claimable", ClaimTokenSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
		ClaimTokenPath: "/worktree/.issueops/state/issueops-v1/lease-2.token",
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got ReseedReceipt
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.ClaimTokenPath != want.ClaimTokenPath || got.Execution.Lease.Generation != 2 || got.Execution.Lease.Status != "claimable" || got.Execution.Workspace.Root != "/worktree" {
		t.Fatalf("round trip=%+v", got)
	}
}
