package issueopspublication

import (
	"testing"

	contract "agent-harness/internal/contract/issueopspublication"
)

func TestDecideReconcile(t *testing.T) {
	tests := []struct {
		name   string
		facts  ReconcileFacts
		action Action
		reason string
	}{
		{name: "one candidate", facts: ReconcileFacts{CandidateCount: 1}, action: ActionAdopt},
		{name: "multiple candidates", facts: ReconcileFacts{CandidateCount: 2}, action: ActionPreserve, reason: "multiple-candidates"},
		{name: "non-authoritative zero", facts: ReconcileFacts{}, action: ActionPreserve, reason: "non-authoritative-zero"},
		{name: "unknown invocation", facts: ReconcileFacts{AuthoritativeZero: true, Invocation: contract.InvocationUnknown}, action: ActionPreserve, reason: "unknown-invocation"},
		{name: "first proven retry", facts: ReconcileFacts{AuthoritativeZero: true, Invocation: contract.InvocationNotInvokedProven}, action: ActionRetry},
		{name: "retry exhausted", facts: ReconcileFacts{AuthoritativeZero: true, Invocation: contract.InvocationNotInvokedProven, RetryCount: 1}, action: ActionPreserve, reason: "retry-exhausted"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := DecideReconcile(test.facts)
			if err != nil {
				t.Fatal(err)
			}
			if got.Action != test.action || got.Reason != test.reason {
				t.Fatalf("got=%#v want action=%q reason=%q", got, test.action, test.reason)
			}
			if got.Action == ActionAdopt && got.CandidateIndex != 0 {
				t.Fatalf("adopt index=%d want=0", got.CandidateIndex)
			}
		})
	}
}

func TestDecideReconcileRejectsNegativeCandidateCount(t *testing.T) {
	got, err := DecideReconcile(ReconcileFacts{CandidateCount: -1})
	if err == nil || err.Error() != "candidate count must not be negative" {
		t.Fatalf("got=%#v err=%v", got, err)
	}
}

func TestValidateCreateEligibility(t *testing.T) {
	tests := []struct {
		name    string
		facts   contract.CreateEligibility
		reason  EligibilityReason
		wantErr string
	}{
		{name: "github pull request", facts: validEligibility("github", "pr", true)},
		{name: "gitlab merge request", facts: validEligibility("gitlab", "mr", true)},
		{name: "unsupported provider", facts: mutateEligibility(validEligibility("github", "pr", true), func(f *contract.CreateEligibility) { f.Provider = "other" }), reason: "provider", wantErr: "publication eligibility: provider"},
		{name: "github kind mismatch", facts: validEligibility("github", "mr", true), reason: "kind", wantErr: "publication eligibility: kind"},
		{name: "gitlab kind mismatch", facts: validEligibility("gitlab", "pr", true), reason: "kind", wantErr: "publication eligibility: kind"},
		{name: "wrong phase", facts: mutateEligibility(validEligibility("github", "pr", true), func(f *contract.CreateEligibility) { f.PhasePR = false }), reason: "phase", wantErr: "publication eligibility: phase"},
		{name: "artifact exists", facts: mutateEligibility(validEligibility("github", "pr", true), func(f *contract.CreateEligibility) { f.NoArtifact = false }), reason: "artifact", wantErr: "publication eligibility: artifact"},
		{name: "branch authority missing", facts: mutateEligibility(validEligibility("github", "pr", true), func(f *contract.CreateEligibility) { f.BranchAuthority = false }), reason: "branch-authority", wantErr: "publication eligibility: branch-authority"},
		{name: "canonical labels and assignees missing", facts: mutateEligibility(validEligibility("github", "pr", true), func(f *contract.CreateEligibility) { f.CanonicalLabelsAssignees = false }), reason: "labels-assignees", wantErr: "publication eligibility: labels-assignees"},
		{name: "confirm execution inactive", facts: mutateEligibility(validEligibility("github", "pr", true), func(f *contract.CreateEligibility) { f.ExecutionActive = false }), reason: "execution", wantErr: "publication eligibility: execution"},
		{name: "confirm pending exists", facts: mutateEligibility(validEligibility("github", "pr", true), func(f *contract.CreateEligibility) { f.NoPending = false }), reason: "pending", wantErr: "publication eligibility: pending"},
		{name: "preview skips execution and pending gates", facts: contract.CreateEligibility{Provider: "github", Kind: "pr", PhasePR: true, NoArtifact: true, BranchAuthority: true, CanonicalLabelsAssignees: true}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reason, err := ValidateCreateEligibility(test.facts)
			if reason != test.reason {
				t.Fatalf("reason=%q want=%q", reason, test.reason)
			}
			if test.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || err.Error() != test.wantErr {
				t.Fatalf("err=%v want=%q", err, test.wantErr)
			}
		})
	}
}

func TestDecideRetryOutcome(t *testing.T) {
	tests := []struct {
		name  string
		facts RetryOutcomeFacts
		want  RetryOutcome
	}{
		{name: "successful call advances to verification", facts: RetryOutcomeFacts{Invocation: contract.InvocationUnknown}, want: RetryOutcomeVerify},
		{name: "proven non-invocation is terminal", facts: RetryOutcomeFacts{Invocation: contract.InvocationNotInvokedProven, CallFailed: true}, want: RetryOutcomeTerminalNotInvoked},
		{name: "unknown failure preserves retry", facts: RetryOutcomeFacts{Invocation: contract.InvocationUnknown, CallFailed: true}, want: RetryOutcomePreserve},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DecideRetryOutcome(test.facts); got != test.want {
				t.Fatalf("got=%q want=%q", got, test.want)
			}
		})
	}
}

func validEligibility(provider, kind string, confirm bool) contract.CreateEligibility {
	return contract.CreateEligibility{
		Provider: provider, Kind: kind, Confirm: confirm, PhasePR: true,
		ExecutionActive: true, NoPending: true, NoArtifact: true,
		BranchAuthority: true, CanonicalLabelsAssignees: true,
	}
}

func mutateEligibility(facts contract.CreateEligibility, mutate func(*contract.CreateEligibility)) contract.CreateEligibility {
	mutate(&facts)
	return facts
}
