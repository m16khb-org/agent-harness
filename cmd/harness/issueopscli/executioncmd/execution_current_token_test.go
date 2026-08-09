package executioncmd

import (
	"strings"
	"testing"
)

func TestClaimRejectsAmbiguousCurrentAndFileTokenSelectors(t *testing.T) {
	err := runClaim([]string{
		"--id", "io-439", "--generation", "2", "--claim-current-token", "--claim-token-file", "/tmp/token", "--json",
	}, Deps{})
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("ambiguous claim token selectors error = %v, want exactly-one rejection", err)
	}
}
