package policy

import (
	"reflect"
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

func TestRedactFreeformRedactsKnownBareTokenFormats(t *testing.T) {
	values := []string{
		"gh" + "p_abcdefghijklmnopqrstuvwxyz123456",
		"github_" + "pat_abcdefghijklmnopqrstuvwxyz123456",
		"xox" + "b-123456789012-abcdefghijklmnop",
		"sk" + "-abcdefghijklmnopqrstuvwxyz123456",
		"AK" + "IAABCDEFGHIJKLMNOP",
	}
	for _, value := range values {
		if got := RedactFreeform(value); got != "<redacted>" {
			t.Fatalf("RedactFreeform(%q) = %q", value, got)
		}
	}
}

// CleanEnvAllowlist/ValidEnvName/RedactArgv는 감사 로그 env allowlist와 argv
// redaction의 진입 규칙이다.
func TestCleanEnvAllowlistTrimsSortsDedupesNothing(t *testing.T) {
	got := CleanEnvAllowlist([]string{" PATH ", "", "HOME", "PATH", "  "})
	want := []string{"HOME", "PATH", "PATH"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("allowlist = %v want %v", got, want)
	}
	if got := CleanEnvAllowlist(nil); len(got) != 0 {
		t.Fatalf("nil allowlist must be empty: %v", got)
	}
}

func TestValidEnvName(t *testing.T) {
	for _, ok := range []string{"PATH", "HOME", "_FOO", "a", "A_1_b"} {
		if !ValidEnvName(ok) {
			t.Fatalf("%q must be a valid env name", ok)
		}
	}
	for _, bad := range []string{"", "1PATH", "-FOO", "FO O", "FOO-BAR", "한글"} {
		if ValidEnvName(bad) {
			t.Fatalf("%q must be invalid", bad)
		}
	}
}

func TestRedactArgvRedactsOnlySecretArgs(t *testing.T) {
	argv := []string{"git", "push", "--token=abc123", "/repo/credentials.json", "origin"}
	got := RedactArgv(argv)
	if got[0] != "git" || got[2] != "<redacted>" || got[3] != "<redacted>" || got[4] != "origin" {
		t.Fatalf("argv redaction wrong: %v", got)
	}
	if got := RedactArgv(nil); len(got) != 0 {
		t.Fatalf("nil argv must produce an empty slice: %v", got)
	}
}
