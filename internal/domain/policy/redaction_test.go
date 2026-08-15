package policy

import (
	"strings"
	"testing"
)

func TestBoundedDiagnosticRedactsAndCapsExternalText(t *testing.T) {
	if got := BoundedDiagnostic("token=private-value", 32); got != "<redacted>" {
		t.Fatalf("redacted diagnostic = %q", got)
	}
	got := BoundedDiagnostic(strings.Repeat("x", 64), 16)
	if got != strings.Repeat("x", 16)+"...[truncated]" {
		t.Fatalf("bounded diagnostic = %q", got)
	}
}
