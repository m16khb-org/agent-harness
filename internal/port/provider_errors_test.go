package port

import (
	"errors"
	"testing"
)

func TestIssueProviderCreateErrorContract(t *testing.T) {
	inner := errors.New("gh auth failed")
	wrapped := &IssueProviderCreateError{Invoked: true, Err: inner}
	if wrapped.Error() == "" || !errors.Is(wrapped, inner) {
		t.Fatalf("create error contract broken: %v", wrapped)
	}
	if got := errors.Unwrap(wrapped); !errors.Is(got, inner) {
		t.Fatalf("unwrap must expose inner: %v", got)
	}
}
