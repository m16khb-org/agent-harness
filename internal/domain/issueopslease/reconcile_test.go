package issueopslease

import "testing"

func TestPlanReconcileStage(t *testing.T) {
	tests := []struct {
		name   string
		input  ReconcileStageRequest
		action ReconcileStageAction
		reason string
	}{
		{name: "exact candidate is adopted", input: ReconcileStageRequest{CandidateCount: 1}, action: ReconcileStageAdopt},
		{name: "multiple candidates are preserved", input: ReconcileStageRequest{CandidateCount: 2}, action: ReconcileStagePreserve, reason: "multiple-candidates"},
		{name: "non authoritative zero is preserved", input: ReconcileStageRequest{AuthoritativeZero: false}, action: ReconcileStagePreserve, reason: "non-authoritative-zero"},
		{name: "unknown create outcome is preserved", input: ReconcileStageRequest{Stage: "task_create", AuthoritativeZero: true, InvocationState: "unknown"}, action: ReconcileStagePreserve, reason: "unknown-invocation"},
		{name: "first proven absent invocation is allowed", input: ReconcileStageRequest{Stage: "task_create", AuthoritativeZero: true, InvocationState: "not_invoked_proven", InvocationAttempts: 0}, action: ReconcileStageInvoke},
		{name: "one proven absent retry is allowed", input: ReconcileStageRequest{Stage: "task_create", AuthoritativeZero: true, InvocationState: "not_invoked_proven", InvocationAttempts: 1}, action: ReconcileStageInvoke},
		{name: "second retry is exhausted", input: ReconcileStageRequest{Stage: "task_create", AuthoritativeZero: true, InvocationState: "not_invoked_proven", InvocationAttempts: 2}, action: ReconcileStagePreserve, reason: "retry-exhausted"},
		{name: "unknown run bind is safely retried", input: ReconcileStageRequest{Stage: "run_bind", AuthoritativeZero: true, InvocationState: "unknown", InvocationAttempts: 1}, action: ReconcileStageInvoke},
		{name: "run bind retry still has the shared bound", input: ReconcileStageRequest{Stage: "run_bind", AuthoritativeZero: true, InvocationState: "unknown", InvocationAttempts: 2}, action: ReconcileStagePreserve, reason: "retry-exhausted"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := PlanReconcileStage(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Action != test.action || plan.Reason != test.reason {
				t.Fatalf("plan = %#v, want action=%q reason=%q", plan, test.action, test.reason)
			}
		})
	}
}

func TestPlanReconcileStageRejectsNegativeCandidateCount(t *testing.T) {
	if _, err := PlanReconcileStage(ReconcileStageRequest{CandidateCount: -1}); err == nil {
		t.Fatal("negative candidate count must fail")
	}
}
