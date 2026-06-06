package policy

import (
	"fmt"
	"testing"
)

func TestPolicyDeniedErrorFormatsDefaultAndJoinedReasons(t *testing.T) {
	empty := PolicyDeniedError{}
	if got, want := empty.Error(), "command denied by policy"; got != want {
		t.Fatalf("empty PolicyDeniedError = %q, want %q", got, want)
	}

	denied := PolicyDeniedError{Reasons: []string{"network denied", "shell denied"}}
	if got, want := denied.Error(), "command denied by policy: network denied; shell denied"; got != want {
		t.Fatalf("PolicyDeniedError = %q, want %q", got, want)
	}
}

func TestIsPolicyDeniedMatchesDirectErrorOnly(t *testing.T) {
	if !IsPolicyDenied(PolicyDeniedError{}) {
		t.Fatal("expected direct PolicyDeniedError to match")
	}
	if IsPolicyDenied(fmt.Errorf("wrapped: %w", PolicyDeniedError{})) {
		t.Fatal("wrapped PolicyDeniedError is not matched by the current direct type assertion contract")
	}
	if IsPolicyDenied(fmt.Errorf("other error")) {
		t.Fatal("non-policy error should not match")
	}
}
