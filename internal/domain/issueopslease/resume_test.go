package issueopslease

import (
	"strings"
	"testing"
)

func TestPlanResumeDecisionTable(t *testing.T) {
	base := ResumeRequest{
		ExpectedGeneration: 3,
		Lease:              Lease{Generation: 3, Status: "claimable", ClaimTokenSHA256: strings.Repeat("a", 64)},
		BindingGeneration:  3,
		BindingRuntimeID:   "runtime",
		BindingTerminalID:  "pty-3",
		CanonicalCWD:       true,
		ModeOrca:           true,
		BindingPresent:     true,
		PendingAbsent:      true,
		RuntimeCompatible:  true,
		Inventory:          ResumeInventory{RuntimeID: "runtime", TerminalID: "pty-3"},
	}
	for _, test := range []struct {
		name string
		edit func(*ResumeRequest)
		want ResumeDisposition
		deny DenyCode
	}{
		{name: "same generation live owner", edit: func(r *ResumeRequest) { r.Inventory.TerminalLive = true; r.Inventory.TaskLive = true }, want: ResumeExistingBinding},
		{name: "contradictory owner", edit: func(r *ResumeRequest) { r.Inventory.TaskLive = true }, deny: DenyResumeOwnerContradiction},
		{name: "previous owner live", edit: func(r *ResumeRequest) {
			r.BindingGeneration = 2
			r.Inventory.TerminalLive = true
			r.Inventory.TaskLive = true
		}, deny: DenyResumeOwnerLive},
		{name: "reuse terminal", edit: func(r *ResumeRequest) { r.Inventory.TerminalLive = true }, want: ResumeReuseTerminal},
		{name: "changed terminal", edit: func(r *ResumeRequest) { r.Inventory.TerminalLive = true; r.Inventory.TerminalID = "other" }, deny: DenyResumeTerminalIdentity},
		{name: "new terminal", edit: func(r *ResumeRequest) {}, want: ResumeCreateTerminal},
		{name: "runtime mismatch", edit: func(r *ResumeRequest) { r.RuntimeCompatible = false }, deny: DenyResumeRuntimeIdentity},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := base
			test.edit(&request)
			plan, err := PlanResume(request)
			if test.deny != "" {
				if DenyCodeOf(err) != test.deny {
					t.Fatalf("denial=%v code=%q want=%q", err, DenyCodeOf(err), test.deny)
				}
				return
			}
			if err != nil || plan.Disposition != test.want {
				t.Fatalf("plan=%#v err=%v want=%q", plan, err, test.want)
			}
		})
	}
}

func TestPlanResumeStageDecisionTable(t *testing.T) {
	for _, test := range []struct {
		name    string
		request ResumeStageRequest
		want    ResumeStageAction
		deny    bool
	}{
		{name: "adopt candidate", request: ResumeStageRequest{CandidateCount: 1}, want: ResumeStageAdopt},
		{name: "multiple candidates", request: ResumeStageRequest{CandidateCount: 2}, deny: true},
		{name: "non authoritative zero", request: ResumeStageRequest{}, want: ResumeStageReconcile},
		{name: "unknown invocation", request: ResumeStageRequest{AuthoritativeZero: true, InvocationState: "unknown"}, want: ResumeStageReconcile},
		{name: "invoke", request: ResumeStageRequest{AuthoritativeZero: true, InvocationState: "not_invoked_proven"}, want: ResumeStageInvoke},
		{name: "exhausted", request: ResumeStageRequest{AuthoritativeZero: true, InvocationState: "not_invoked_proven", InvocationAttempts: 2}, deny: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan, err := PlanResumeStage(test.request)
			if test.deny {
				if err == nil {
					t.Fatal("expected denial")
				}
				return
			}
			if err != nil || plan.Action != test.want {
				t.Fatalf("plan=%#v err=%v want=%q", plan, err, test.want)
			}
		})
	}
}
