package issueops

import (
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
)

func TestIssueOpsChildStatusAggregatesAndRepairs(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	first, err := startIssueOpsChildForTest(stateRoot, parent, issueops.IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "123-child-status-a",
		Title:              "status child a",
		TaskScope:          "status child a",
		AcceptanceCriteria: []string{"status includes indexed child"},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := startIssueOpsChildForTest(stateRoot, parent, issueops.IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "123-child-status-b",
		Title:              "status child b",
		TaskScope:          "status child b",
		AcceptanceCriteria: []string{"status scans missing child ref"},
	})
	if err != nil {
		t.Fatal(err)
	}
	parentAfter, err := ReadIssueOps(stateRoot, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	parentAfter.ChildCycles = []issueops.IssueOpsChildCycleRef{
		first.ParentRef,
		{CycleID: "io-deadbeefcafe", Branch: "999-orphan-child", Title: "orphan child", CreatedAt: "2026-07-07T00:00:00Z"},
	}
	writeIssueOpsRecordForDelegationTest(t, stateRoot, parentAfter)

	status, err := IssueOpsChildStatus(stateRoot, parent.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Children) != 3 {
		t.Fatalf("expected indexed, scanned, and orphaned child entries, got %#v", status.Children)
	}
	scanned, ok := childStatusEntryByID(status.Children, second.Child.ID)
	if !ok || !scanned.Scanned || scanned.Indexed || scanned.Orphaned || scanned.Phase != IssueOpsPhaseProblem {
		t.Fatalf("missing-index child should be surfaced from child delegation pointer, got %#v", scanned)
	}
	orphan, ok := childStatusEntryByID(status.Children, "io-deadbeefcafe")
	if !ok || !orphan.Indexed || orphan.Scanned || !orphan.Orphaned {
		t.Fatalf("orphaned parent ref should be marked, got %#v", orphan)
	}
	if status.Repaired || len(status.RepairAppended) != 0 {
		t.Fatalf("repair=false should not mutate parent index, got %#v", status)
	}

	status, err = IssueOpsChildStatusWithActor(stateRoot, parent.ID, true, issueOpsActorForTest(parent.WorktreePath))
	if err != nil {
		t.Fatal(err)
	}
	if !status.Repaired || len(status.RepairAppended) != 1 || status.RepairAppended[0] != second.Child.ID {
		t.Fatalf("repair=true should append missing scanned child ref, got %#v", status)
	}
	repairedParent, err := ReadIssueOps(stateRoot, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !hasChildRef(repairedParent.ChildCycles, second.Child.ID) {
		t.Fatalf("repair should persist scanned child ref on parent: %#v", repairedParent.ChildCycles)
	}
}

func TestAcceptIssueOpsChildRequiresDonePhaseAndEvidence(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	started, err := startIssueOpsChildForTest(stateRoot, parent, issueops.IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "123-child-accept",
		Title:              "accept child",
		TaskScope:          "validate accept gate",
		AcceptanceCriteria: []string{"accept requires done and evidence"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acceptIssueOpsChildForTest(stateRoot, parent, started.Child.ID, []string{"tests passed"}); err == nil || !strings.Contains(err.Error(), "child_not_done") {
		t.Fatalf("accept should refuse non-done child, got %v", err)
	}
	child := started.Child
	child.Phase = IssueOpsPhaseDone
	writeIssueOpsRecordForDelegationTest(t, stateRoot, child)
	if _, err := acceptIssueOpsChildForTest(stateRoot, parent, child.ID, nil); err == nil || !strings.Contains(err.Error(), "validation_evidence") {
		t.Fatalf("accept should require evidence, got %v", err)
	}
	result, err := acceptIssueOpsChildForTest(stateRoot, parent, child.ID, []string{"parent verified merged child diff"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ParentRef.ValidationVerdict != "accepted" || len(result.ParentRef.ValidationEvidence) != 1 || result.ParentRef.ValidatedAt == "" {
		t.Fatalf("accept should record verdict/evidence/timestamp on parent ref: %#v", result.ParentRef)
	}
	parentAfter, err := ReadIssueOps(stateRoot, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	ref, ok := childRefByID(parentAfter.ChildCycles, child.ID)
	if !ok || ref.ValidationVerdict != "accepted" || ref.ValidationReason != "" {
		t.Fatalf("accepted verdict should persist on parent ref without reason, got %#v", ref)
	}

	// cleanup finish가 child 레코드를 삭제해도 accepted parent receipt는 완료
	// 증거로 남아야 한다. 이를 orphan/incomplete로 되돌리면 parent PR gate가
	// 이미 승인·정리한 child 때문에 영구적으로 막힌다.
	if err := deleteIssueOps(stateRoot, child.ID); err != nil {
		t.Fatal(err)
	}
	status, err := IssueOpsChildStatus(stateRoot, parent.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	archived, ok := childStatusEntryByID(status.Children, child.ID)
	if !ok || archived.Orphaned || archived.Phase != IssueOpsPhaseDone || len(status.Orphaned) != 0 {
		t.Fatalf("accepted cleanup receipt must stay terminal without becoming orphaned, got %#v / %#v", archived, status.Orphaned)
	}
	if missing, notes := issueOpsChildPRGateMissing(stateRoot, parentAfter); len(missing) != 0 || len(notes) != 0 {
		t.Fatalf("accepted cleanup receipt must not block the parent PR gate: missing=%v notes=%v", missing, notes)
	}
}

func TestAcceptIssueOpsChildAfterCleanupUsesIndexedParentRef(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	started, err := startIssueOpsChildForTest(stateRoot, parent, issueops.IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "123-cleaned-child-accept",
		Title:              "cleaned child accept",
		TaskScope:          "validate cleaned child result",
		AcceptanceCriteria: []string{"parent can accept verified cleaned child"},
	})
	if err != nil {
		t.Fatal(err)
	}
	child := started.Child
	child.Phase = IssueOpsPhaseDone
	writeIssueOpsRecordForDelegationTest(t, stateRoot, child)
	if err := deleteIssueOps(stateRoot, child.ID); err != nil {
		t.Fatal(err)
	}

	result, err := acceptIssueOpsChildForTest(stateRoot, parent, child.ID, []string{"merged result verified after cleanup"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ParentRef.ValidationVerdict != "accepted" || result.ParentRef.ValidatedAt == "" {
		t.Fatalf("cleaned child acceptance should persist a parent receipt: %#v", result.ParentRef)
	}
	status, err := IssueOpsChildStatus(stateRoot, parent.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	accepted, ok := childStatusEntryByID(status.Children, child.ID)
	if !ok || accepted.Orphaned || accepted.Phase != IssueOpsPhaseDone {
		t.Fatalf("accepted cleaned child should stay terminal without orphaning: %#v", accepted)
	}
}

func TestAcceptIssueOpsChildAfterCleanupRejectsUnindexedID(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)

	_, err := acceptIssueOpsChildForTest(stateRoot, parent, "io-deadbeefcafe", []string{"external evidence"})
	if err == nil || !strings.Contains(err.Error(), "child_not_indexed") {
		t.Fatalf("missing child without a parent ref must fail closed: %v", err)
	}
}

func TestRejectIssueOpsChildRecordsVerdictOnValidReason(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	started, err := startIssueOpsChildForTest(stateRoot, parent, issueops.IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "123-child-reject",
		Title:              "reject child",
		TaskScope:          "validate reject verdict",
		AcceptanceCriteria: []string{"reject records parent-owned verdict"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rejectIssueOpsChildForTest(stateRoot, parent, started.Child.ID, "too short", []string{"needs redo"}); err == nil || !strings.Contains(err.Error(), "reason") {
		t.Fatalf("reject should require a reason with at least 10 chars, got %v", err)
	}
	result, err := rejectIssueOpsChildForTest(stateRoot, parent, started.Child.ID, "missing required integration tests", []string{"unit tests absent"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ParentRef.ValidationVerdict != "rejected" || result.ParentRef.ValidationReason != "missing required integration tests" || len(result.ParentRef.ValidationEvidence) != 1 || result.ParentRef.ValidatedAt == "" {
		t.Fatalf("reject should record verdict/reason/evidence/timestamp: %#v", result.ParentRef)
	}
}

func TestDropIssueOpsChildRecordsAuditTrail(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	started, err := startIssueOpsChildForTest(stateRoot, parent, issueops.IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "123-child-drop",
		Title:              "drop child",
		TaskScope:          "validate drop verdict",
		AcceptanceCriteria: []string{"drop records audited reason"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dropIssueOpsChildForTest(stateRoot, parent, started.Child.ID, "too short"); err == nil || !strings.Contains(err.Error(), "reason") {
		t.Fatalf("drop should require a reason with at least 10 chars, got %v", err)
	}
	result, err := dropIssueOpsChildForTest(stateRoot, parent, started.Child.ID, "scope intentionally removed from parent plan")
	if err != nil {
		t.Fatal(err)
	}
	if result.ParentRef.ValidationVerdict != "dropped" || result.ParentRef.ValidationReason != "scope intentionally removed from parent plan" || result.ParentRef.ValidatedAt == "" {
		t.Fatalf("drop should record verdict/reason/timestamp: %#v", result.ParentRef)
	}
}

func TestDropIssueOpsChildAfterCleanupUsesIndexedParentRef(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	started, err := startIssueOpsChildForTest(stateRoot, parent, issueops.IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "123-cleaned-child-drop",
		Title:              "cleaned child drop",
		TaskScope:          "recover an intentionally abandoned child",
		AcceptanceCriteria: []string{"parent can drop an indexed child after cleanup"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := deleteIssueOps(stateRoot, started.Child.ID); err != nil {
		t.Fatal(err)
	}

	result, err := dropIssueOpsChildForTest(stateRoot, parent, started.Child.ID, "execution was reconciled and intentionally abandoned")
	if err != nil {
		t.Fatal(err)
	}
	if result.ParentRef.ValidationVerdict != "dropped" || result.ParentRef.ValidationReason == "" || result.ParentRef.ValidatedAt == "" {
		t.Fatalf("cleaned child drop should persist an audited parent receipt: %#v", result.ParentRef)
	}
	status, err := IssueOpsChildStatus(stateRoot, parent.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	dropped, ok := childStatusEntryByID(status.Children, started.Child.ID)
	if !ok || dropped.Orphaned || dropped.ValidationVerdict != "dropped" || len(status.Orphaned) != 0 {
		t.Fatalf("dropped cleaned child should not remain orphaned: %#v / %#v", dropped, status.Orphaned)
	}
	parentAfter, err := ReadIssueOps(stateRoot, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if missing, notes := issueOpsChildPRGateMissing(stateRoot, parentAfter); len(missing) != 0 || len(notes) != 0 {
		t.Fatalf("dropped cleaned child must not block the parent PR gate: missing=%v notes=%v", missing, notes)
	}

	_, err = dropIssueOpsChildForTest(stateRoot, parent, "io-deadbeefcafe", "unindexed records must fail closed")
	if err == nil || !strings.Contains(err.Error(), "child_not_indexed") {
		t.Fatalf("missing child without a parent ref must fail closed: %v", err)
	}
}

func TestDroppedCleanupReceiptWithShortReasonRemainsOrphaned(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	started, err := startIssueOpsChildForTest(stateRoot, parent, issueops.IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "123-malformed-drop-receipt",
		Title:              "malformed drop receipt",
		TaskScope:          "reject malformed archived drop receipts",
		AcceptanceCriteria: []string{"short drop reasons remain fail closed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := deleteIssueOps(stateRoot, started.Child.ID); err != nil {
		t.Fatal(err)
	}
	parentAfter, err := ReadIssueOps(stateRoot, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i := range parentAfter.ChildCycles {
		if parentAfter.ChildCycles[i].CycleID == started.Child.ID {
			parentAfter.ChildCycles[i].ValidationVerdict = "dropped"
			parentAfter.ChildCycles[i].ValidationReason = "short"
			parentAfter.ChildCycles[i].ValidatedAt = "2026-08-09T00:00:00Z"
		}
	}
	writeIssueOpsRecordForDelegationTest(t, stateRoot, parentAfter)

	status, err := IssueOpsChildStatus(stateRoot, parent.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := childStatusEntryByID(status.Children, started.Child.ID)
	if !ok || !entry.Orphaned || len(status.Orphaned) != 1 {
		t.Fatalf("malformed dropped receipt must remain orphaned: %#v / %#v", entry, status.Orphaned)
	}
	if missing, notes := issueOpsChildPRGateMissing(stateRoot, parentAfter); len(missing) == 0 || len(notes) != 0 {
		t.Fatalf("malformed dropped receipt must block the parent PR gate: missing=%v notes=%v", missing, notes)
	}
}

func childStatusEntryByID(entries []issueops.IssueOpsChildStatusEntry, childID string) (issueops.IssueOpsChildStatusEntry, bool) {
	for _, entry := range entries {
		if entry.CycleID == childID {
			return entry, true
		}
	}
	return issueops.IssueOpsChildStatusEntry{}, false
}
