package issueops

import (
	"testing"

	"agent-harness/internal/contract/issueops"
)

func TestIssueOpsStrictPRReadinessBlocksIncompleteChildren(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	started, err := startIssueOpsChildForTest(stateRoot, parent, issueops.IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "123-child-pr-incomplete",
		Title:              "incomplete child",
		TaskScope:          "prove parent pr gate blocks incomplete child",
		AcceptanceCriteria: []string{"parent waits for child completion"},
	})
	if err != nil {
		t.Fatal(err)
	}

	ready := issueOpsStrictPRReadinessWithState(stateRoot, parent)
	if !containsString(ready.Missing, "child_incomplete:"+started.Child.ID) {
		t.Fatalf("parent pr gate should block incomplete child, got %#v", ready.Missing)
	}

	child := started.Child
	child.Phase = IssueOpsPhaseDone
	writeIssueOpsRecordForDelegationTest(t, stateRoot, child)
	ready = issueOpsStrictPRReadinessWithState(stateRoot, parent)
	if containsString(ready.Missing, "child_incomplete:"+child.ID) || !containsString(ready.Missing, "child_unvalidated:"+child.ID) {
		t.Fatalf("done child without verdict should be unvalidated only, got %#v", ready.Missing)
	}

	if _, err := acceptIssueOpsChildForTest(stateRoot, parent, child.ID, []string{"parent verified child output"}); err != nil {
		t.Fatal(err)
	}
	parentAfter, err := ReadIssueOps(stateRoot, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	ready = issueOpsStrictPRReadinessWithState(stateRoot, parentAfter)
	if containsString(ready.Missing, "child_incomplete:"+child.ID) || containsString(ready.Missing, "child_unvalidated:"+child.ID) {
		t.Fatalf("accepted done child should clear child pr gate keys, got %#v", ready.Missing)
	}
}

func TestIssueOpsStrictPRReadinessRejectedAndDroppedVerdicts(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	started, err := startIssueOpsChildForTest(stateRoot, parent, issueops.IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "123-child-pr-rejected",
		Title:              "rejected child",
		TaskScope:          "prove rejected child keeps parent blocked",
		AcceptanceCriteria: []string{"rejected child blocks until dropped"},
	})
	if err != nil {
		t.Fatal(err)
	}
	child := started.Child
	child.Phase = IssueOpsPhaseDone
	writeIssueOpsRecordForDelegationTest(t, stateRoot, child)
	if _, err := rejectIssueOpsChildForTest(stateRoot, parent, child.ID, "needs another integration pass", []string{"missing validation"}); err != nil {
		t.Fatal(err)
	}
	parentAfter, err := ReadIssueOps(stateRoot, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	ready := issueOpsStrictPRReadinessWithState(stateRoot, parentAfter)
	if !containsString(ready.Missing, "child_rejected_unresolved:"+child.ID) {
		t.Fatalf("rejected child should keep parent pr gate blocked, got %#v", ready.Missing)
	}

	if _, err := dropIssueOpsChildForTest(stateRoot, parent, child.ID, "work deliberately removed from scope"); err != nil {
		t.Fatal(err)
	}
	parentAfter, err = ReadIssueOps(stateRoot, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	ready = issueOpsStrictPRReadinessWithState(stateRoot, parentAfter)
	if containsString(ready.Missing, "child_rejected_unresolved:"+child.ID) || containsString(ready.Missing, "child_unvalidated:"+child.ID) {
		t.Fatalf("dropped child should clear child pr gate keys, got %#v", ready.Missing)
	}
}

func TestIssueOpsParentWithoutChildrenUnaffected(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	ready := issueOpsStrictPRReadinessWithState(stateRoot, parent)
	for _, missing := range ready.Missing {
		if len(missing) >= len("child_") && missing[:len("child_")] == "child_" {
			t.Fatalf("parent with no child cycles should not gain child gate keys, got %#v", ready.Missing)
		}
	}
}
