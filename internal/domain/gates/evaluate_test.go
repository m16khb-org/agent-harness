package gates

import (
	"strings"
	"testing"
)

func TestExpectMatchesLineAnchored(t *testing.T) {
	if !ExpectMatches("3/3 tiers ok", "checking...\n3/3 tiers ok\n") {
		t.Fatal("whole-line match failed")
	}
	if !ExpectMatches("ok", "ok  \tagent-harness/internal/adapter/gates\t0.3s") {
		t.Fatal("EXPECT followed by whitespace must match (go test lines)")
	}
	if !ExpectMatches("docs-ok", "Skill is valid!\n  docs-ok  \n") {
		t.Fatal("trimmed line equal to EXPECT must match")
	}
	if ExpectMatches("docs-ok", "echo: error: SKILL.md not found\ndocs-ok: error: SKILL.md not found") {
		t.Fatal("EXPECT followed by ':' inside an error line must not match (#484)")
	}
	if ExpectMatches("ok", "suite: 8/8 passed ok\nbroken") {
		t.Fatal("EXPECT in the middle of a line must not match; use /regex/ for that")
	}
	if ExpectMatches("done", "starting...") {
		t.Fatal("absent text must not match")
	}
}

func TestExpectMatchesRegexForm(t *testing.T) {
	if !ExpectMatches("/8\\/8 passed/", "suite: 8/8 passed") {
		t.Fatal("escaped regex match failed")
	}
	if !ExpectMatches("/all (ok|good)/i", "ALL GOOD") {
		t.Fatal("case-insensitive flag failed")
	}
	if ExpectMatches("/ok/", "no match here") {
		t.Fatal("regex matched absent text")
	}
	if ExpectMatches("/([unclosed/", "anything") {
		t.Fatal("invalid regex must evaluate false, not panic")
	}
	if !ExpectMatches("/^first$/m", "ignored\nfirst\nlast") {
		t.Fatal("multiline flag failed")
	}
}

func TestEvidenceTailKeepsLastTwoNonEmptyLines(t *testing.T) {
	got := EvidenceTail("line1\nline2\n\nline3\n  line4  \n", 200)
	if got != "line3 | line4" {
		t.Fatalf("tail = %q, want %q", got, "line3 | line4")
	}
	if got := EvidenceTail("", 200); got != "(no output)" {
		t.Fatalf("empty tail = %q", got)
	}
	long := EvidenceTail(strings.Repeat("x", 300), 200)
	if len(long) != 200 {
		t.Fatalf("tail cap = %d, want 200", len(long))
	}
}

func TestEvidencePending(t *testing.T) {
	for _, pending := range []string{"", "pending", "PENDING", "  pending  "} {
		if !EvidencePending(pending) {
			t.Fatalf("%q must be pending", pending)
		}
	}
	if EvidencePending("measured 3/3") {
		t.Fatal("measured evidence must not be pending")
	}
}

func TestStateTransitions(t *testing.T) {
	cases := []struct {
		name string
		gate Gate
		want string
	}{
		{"abandoned wins", Gate{Checked: false, Abandoned: true}, StateAbandoned},
		{"unchecked", Gate{}, StateUnchecked},
		{"checked without evidence", Gate{Checked: true, Evidence: "pending"}, StateEvidencePending},
		{"checked with empty evidence", Gate{Checked: true}, StateEvidencePending},
		{"met", Gate{Checked: true, Evidence: "measured"}, StateMet},
	}
	for _, tc := range cases {
		if got := State(tc.gate); got != tc.want {
			t.Fatalf("%s: state = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestSummarizeCountsAndComplete(t *testing.T) {
	summary := Summarize([]Gate{
		{Checked: true, Evidence: "measured"},
		{},
		{Checked: true, Evidence: "pending"},
		{Abandoned: true, AbandonReason: "reason"},
	})
	if summary.Total != 4 || summary.Met != 1 || summary.Unmet != 2 || summary.Abandoned != 1 {
		t.Fatalf("summary counts wrong: %+v", summary)
	}
	if summary.Complete {
		t.Fatal("unmet>0 must not be complete")
	}
	allResolved := Summarize([]Gate{
		{Checked: true, Evidence: "measured"},
		{Abandoned: true},
	})
	if !allResolved.Complete || allResolved.Met != 1 || allResolved.Abandoned != 1 {
		t.Fatalf("met+abandoned must be complete: %+v", allResolved)
	}
}
