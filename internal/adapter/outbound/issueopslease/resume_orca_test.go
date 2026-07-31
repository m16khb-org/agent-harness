package issueopslease

import (
	"context"
	"testing"

	leaseapp "agent-harness/internal/application/issueopslease"
	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
)

func TestResumeOwnerAndStageAdaptersDelegateOnce(t *testing.T) {
	ownerCalls, inspectCalls, invokeCalls := 0, 0, 0
	owner := NewResumeOwnerInventory(func(context.Context, leasecontract.Record) (leasedomain.ResumeInventory, bool, error) {
		ownerCalls++
		return leasedomain.ResumeInventory{RuntimeID: "runtime"}, true, nil
	})
	stages := NewResumeStageExecutor(
		func(context.Context, leaseapp.ResumeIntentState) (leasecontract.ResumeStageInventory, error) {
			inspectCalls++
			return leasecontract.ResumeStageInventory{AuthoritativeZero: true}, nil
		},
		func(context.Context, leaseapp.ResumeIntentState) (leasecontract.ResumeStageReceipt, error) {
			invokeCalls++
			return leasecontract.ResumeStageReceipt{TerminalPTYID: "pty"}, nil
		},
	)
	if _, compatible, err := owner.Observe(context.Background(), resumeRepositoryRecord(t, 1)); err != nil || !compatible {
		t.Fatalf("owner err=%v compatible=%t", err, compatible)
	}
	if _, err := stages.Inspect(context.Background(), leaseapp.ResumeIntentState{}); err != nil {
		t.Fatal(err)
	}
	if _, err := stages.Invoke(context.Background(), leaseapp.ResumeIntentState{}); err != nil {
		t.Fatal(err)
	}
	if ownerCalls != 1 || inspectCalls != 1 || invokeCalls != 1 {
		t.Fatalf("calls owner=%d inspect=%d invoke=%d", ownerCalls, inspectCalls, invokeCalls)
	}
}
