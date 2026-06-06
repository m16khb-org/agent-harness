package handoff

import "testing"

func TestJoinHandoffArgsQuotesShellSensitiveArguments(t *testing.T) {
	args := []string{"agy", "--flag", "plain", "two words", "it's", "line\nbreak", `"quoted"`}

	got := JoinArgs(args)
	want := `agy --flag plain 'two words' 'it'"'"'s' 'line
break' '"quoted"'`

	if got != want {
		t.Fatalf("joinHandoffArgs = %q, want %q", got, want)
	}
}

func TestStrconvQuoteEscapesSingleQuotes(t *testing.T) {
	got := strconvQuote("can't stop")
	want := `'can'"'"'t stop'`

	if got != want {
		t.Fatalf("strconvQuote = %q, want %q", got, want)
	}
}
