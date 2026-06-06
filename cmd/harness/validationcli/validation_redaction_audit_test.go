package validationcli

import (
	"strings"
	"testing"
)

func TestFindUnredactedSecretLikeFlagsRealTokens(t *testing.T) {
	findings := findUnredactedSecretLike("OPENAI_API_KEY=sk-123456789012345678901234\n")
	if len(findings) == 0 {
		t.Fatal("expected unredacted secret finding")
	}
	if !strings.Contains(findings[0], "openai_token") {
		t.Fatalf("unexpected finding: %+v", findings)
	}
}

func TestFindUnredactedSecretLikeAllowsRedactedFixtures(t *testing.T) {
	findings := findUnredactedSecretLike("TOKEN=redacted\npassword=example\n")
	if len(findings) != 0 {
		t.Fatalf("redacted fixtures should be allowed: %+v", findings)
	}
}
