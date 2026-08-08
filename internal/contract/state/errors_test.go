package state_test

import (
	"errors"
	"testing"

	"agent-harness/internal/contract/state"
)

func TestInvalidExistingStateUsesOnePublicIdentity(t *testing.T) {
	cases := []error{
		state.Invalid("missing_schema"),
		state.Invalid("future_schema"),
		state.Invalid("malformed_json"),
		state.Invalid("legacy_field"),
		state.Invalid("key_mismatch"),
		state.Invalid("byte_mismatch"),
	}
	for _, err := range cases {
		if !errors.Is(err, state.ErrInvalidState) || err.Error() != "invalid state" {
			t.Fatalf("public invalid-state drift: %q", err)
		}
	}
}
