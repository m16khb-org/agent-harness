package guard

import (
	guardcontract "agent-harness/internal/contract/guard"
	"fmt"
	"testing"
)

func TestGuardBlockedErrorFormatsDefaultAndFirstFindingRule(t *testing.T) {
	empty := GuardBlockedError{}
	if got, want := empty.Error(), "guard check blocked"; got != want {
		t.Fatalf("empty GuardBlockedError = %q, want %q", got, want)
	}

	blocked := GuardBlockedError{Findings: []guardcontract.GuardFinding{
		{Rule: "no-secrets"},
		{Rule: "no-large-file"},
	}}
	if got, want := blocked.Error(), "guard check blocked: no-secrets"; got != want {
		t.Fatalf("GuardBlockedError = %q, want %q", got, want)
	}
}

func TestIsGuardBlockedMatchesDirectErrorOnly(t *testing.T) {
	if !IsGuardBlocked(GuardBlockedError{}) {
		t.Fatal("expected direct GuardBlockedError to match")
	}
	if IsGuardBlocked(fmt.Errorf("wrapped: %w", GuardBlockedError{})) {
		t.Fatal("wrapped GuardBlockedError is not matched by the current direct type assertion contract")
	}
	if IsGuardBlocked(fmt.Errorf("other error")) {
		t.Fatal("non-guard error should not match")
	}
}
