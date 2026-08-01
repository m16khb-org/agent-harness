package issueopslease

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestReconcileStageInventoryRoundTrip(t *testing.T) {
	want := ReconcileStageInventory{
		Candidates: []ReconcileStageReceipt{{
			Workspace: &ReconcilePreparedReceipt{
				Workspace: ReconcileWorkspaceReceipt{SourceRoot: "/source", Root: "/worktree", Branch: "194", BaseHead: "abc", Driver: "orca", Exists: true},
				RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", WorktreeInstanceID: "instance",
			},
			TerminalPTYID: "pty", RunID: "run", RunBound: true, TaskID: "task", DispatchID: "dispatch",
		}},
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got ReconcileStageInventory
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip=%#v", got)
	}
}
