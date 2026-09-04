package issueopspublication

import (
	"fmt"

	contract "issueops/internal/contract/issueopspublication"
)

type Action string

const (
	ActionAdopt    Action = "adopt"
	ActionRetry    Action = "retry"
	ActionPreserve Action = "preserve"
)

type ReconcileFacts struct {
	CandidateCount    int
	AuthoritativeZero bool
	Invocation        contract.InvocationState
	RetryCount        int
}

type Decision struct {
	Action         Action
	CandidateIndex int
	Reason         string
}

func DecideReconcile(facts ReconcileFacts) (Decision, error) {
	if facts.CandidateCount < 0 {
		return Decision{}, fmt.Errorf("candidate count must not be negative")
	}
	if facts.CandidateCount > 1 {
		return Decision{Action: ActionPreserve, Reason: "multiple-candidates"}, nil
	}
	if facts.CandidateCount == 1 {
		return Decision{Action: ActionAdopt, CandidateIndex: 0}, nil
	}
	if !facts.AuthoritativeZero {
		return Decision{Action: ActionPreserve, Reason: "non-authoritative-zero"}, nil
	}
	if facts.Invocation != contract.InvocationNotInvokedProven {
		return Decision{Action: ActionPreserve, Reason: "unknown-invocation"}, nil
	}
	if facts.RetryCount != 0 {
		return Decision{Action: ActionPreserve, Reason: "retry-exhausted"}, nil
	}
	return Decision{Action: ActionRetry}, nil
}

type EligibilityReason string

func ValidateCreateEligibility(facts contract.CreateEligibility) (EligibilityReason, error) {
	if facts.Provider != "github" && facts.Provider != "gitlab" {
		return "provider", fmt.Errorf("publication eligibility: provider")
	}
	if (facts.Provider == "github" && facts.Kind != "pr") ||
		(facts.Provider == "gitlab" && facts.Kind != "mr") {
		return "kind", fmt.Errorf("publication eligibility: kind")
	}
	checks := []struct {
		ok     bool
		reason EligibilityReason
	}{
		{ok: facts.PhasePR, reason: "phase"},
		{ok: facts.NoArtifact, reason: "artifact"},
		{ok: facts.BranchAuthority, reason: "branch-authority"},
		{ok: facts.CanonicalLabelsAssignees, reason: "labels-assignees"},
	}
	for _, check := range checks {
		if !check.ok {
			return check.reason, fmt.Errorf("publication eligibility: %s", check.reason)
		}
	}
	if facts.Confirm && !facts.ExecutionActive {
		return "execution", fmt.Errorf("publication eligibility: execution")
	}
	if facts.Confirm && !facts.NoPending {
		return "pending", fmt.Errorf("publication eligibility: pending")
	}
	return "", nil
}

type RetryOutcomeFacts struct {
	Invocation contract.InvocationState
	CallFailed bool
}

type RetryOutcome string

const (
	RetryOutcomeVerify             RetryOutcome = "verify"
	RetryOutcomePreserve           RetryOutcome = "preserve"
	RetryOutcomeTerminalNotInvoked RetryOutcome = "terminal-not-invoked"
)

func DecideRetryOutcome(facts RetryOutcomeFacts) RetryOutcome {
	if !facts.CallFailed {
		return RetryOutcomeVerify
	}
	if facts.Invocation == contract.InvocationNotInvokedProven {
		return RetryOutcomeTerminalNotInvoked
	}
	return RetryOutcomePreserve
}
