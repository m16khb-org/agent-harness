package qagate

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

// "task-N-..." evidence file names contain the substring "sk-N-..."; without
// a word boundary the openai_token pattern false-positives on ordinary docs
// and deterministically fails the 95-gate's redaction audit.
func TestFindUnredactedSecretLikeIgnoresTaskFileNames(t *testing.T) {
	findings := findUnredactedSecretLike("Evidence: .issueops/evidence/task-0-live-invocation-record.md\nEvidence: task-2-pioneer-benchmark-error.txt\n")
	if len(findings) != 0 {
		t.Fatalf("task-N file names must not be flagged as tokens: %+v", findings)
	}
}

func TestFindUnredactedSecretLikeAllowsRedactedFixtures(t *testing.T) {
	findings := findUnredactedSecretLike("TOKEN=redacted\npassword=example\n")
	if len(findings) != 0 {
		t.Fatalf("redacted fixtures should be allowed: %+v", findings)
	}
}
