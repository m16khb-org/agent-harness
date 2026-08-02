package issueops

import (
	"context"
	"testing"

	"agent-harness/internal/contract/issueops"
)

// 게이트를 통과한 레코드는 삭제 CAS에서도 통과해야 한다. 두 곳이 같은 조건을
// 각각 표현하면 preview가 fingerprint까지 발급한 뒤 apply가 거부되는 상태가
// 생긴다 — 운영자에게는 이유 없이 막히는 것으로 보인다(#143, #142에서 실측).
func TestAbandonDeletesHolderlessLeaseRecord(t *testing.T) {
	for _, status := range []issueops.LeaseStatus{issueops.LeaseStatusClaimable, issueops.LeaseStatusReleased} {
		t.Run(string(status), func(t *testing.T) {
			stateRoot, record := abandonLeaseRecord(t, status)
			deps := abandonOrcaDeps(&fakeAbandonGit{}, &fakeOwnerInspector{})

			preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
			if err != nil || preview.Fingerprint == "" {
				t.Fatalf("preview must be ready for a holderless lease: %v %+v", err, preview.Missing)
			}

			applied, err := CleanupAbandon(context.Background(), stateRoot,
				abandonRequest(record.ID, true, preview.Fingerprint), deps)
			if err != nil {
				t.Fatalf("a record the gates cleared must not be refused at the deletion CAS: %v (failed_step=%q)",
					err, applied.FailedStep)
			}
			if !applied.RecordDeleted {
				t.Fatalf("apply must delete the record: %+v", applied)
			}
			if _, err := ReadIssueOps(stateRoot, record.ID); err == nil {
				t.Fatal("the record must be gone after a confirmed abandon")
			}
		})
	}
}

// 삭제 직전 재검사의 본래 목적은 유지된다. fingerprint 계산 이후 권위 필드가
// 실제로 바뀌면 여전히 거부한다.
func TestAbandonDeletionCASStillCatchesAuthorityDrift(t *testing.T) {
	stateRoot, record := abandonLeaseRecord(t, issueops.LeaseStatusClaimable)
	deps := abandonOrcaDeps(&fakeAbandonGit{}, &fakeOwnerInspector{})

	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil || preview.Fingerprint == "" {
		t.Fatalf("preview must be ready: %v %+v", err, preview.Missing)
	}

	// preview 이후 홀더가 생겼다. 이 apply는 다른 상태를 지우는 것이 된다.
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		holder := executionActor("claude", "drifted-session")
		rec.Execution.Lease = issueops.WriteLease{
			Generation: 1, Status: issueops.LeaseStatusActive,
			Holder: &holder, ClaimedAt: "2026-07-26T00:00:03Z",
		}
	})

	if _, err := CleanupAbandon(context.Background(), stateRoot,
		abandonRequest(record.ID, true, preview.Fingerprint), deps); err == nil {
		t.Fatal("a lease that gained a holder after preview must be refused")
	}
	if _, err := ReadIssueOps(stateRoot, record.ID); err != nil {
		t.Fatalf("a refused abandon must preserve the record: %v", err)
	}
}
