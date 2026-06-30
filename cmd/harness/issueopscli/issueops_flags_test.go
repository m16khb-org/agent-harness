package issueopscli

import (
	"flag"
	"strings"
	"testing"
)

// TestRepeatedFlagRoundTripsRepeatedValues exercises the canonical repeatable
// string flag through a real FlagSet: each --flag occurrence appends one value
// and String() joins the collected values with the single "," separator.
func TestRepeatedFlagRoundTripsRepeatedValues(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var values repeatedFlag
	fs.Var(&values, "item", "repeatable item")
	if err := fs.Parse([]string{"--item", "a", "--item", "b", "--item", "c"}); err != nil {
		t.Fatalf("parse repeated flag: %v", err)
	}
	if got, want := []string(values), []string{"a", "b", "c"}; !equalStringSlices(got, want) {
		t.Fatalf("collected values = %v, want %v", got, want)
	}
	if got, want := values.String(), "a,b,c"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

// TestSliceFlagIsRepeatedFlagAlias proves the duplicate flag type collapsed to a
// single canonical type: sliceFlag is an alias of repeatedFlag, so it shares the
// same Set/String behavior and the same "," separator.
func TestSliceFlagIsRepeatedFlagAlias(t *testing.T) {
	var alias sliceFlag
	if err := alias.Set("x"); err != nil {
		t.Fatalf("alias.Set: %v", err)
	}
	if err := alias.Set("y"); err != nil {
		t.Fatalf("alias.Set: %v", err)
	}
	if got, want := alias.String(), "x,y"; got != want {
		t.Fatalf("alias String() = %q, want %q", got, want)
	}
	// A repeatedFlag value is assignable to a sliceFlag variable (and vice
	// versa) only because they are the same type, not merely convertible.
	var canonical repeatedFlag = alias
	if got, want := canonical.String(), "x,y"; got != want {
		t.Fatalf("canonical String() = %q, want %q", got, want)
	}
}

// TestIssueOpsUsageListsNewlyAddedSubcommands guards the usage text against the
// docs-drift regression: every subcommand registered in issueOpsSubcommands that
// the audit found missing must appear in issueOpsUsage().
func TestIssueOpsUsageListsNewlyAddedSubcommands(t *testing.T) {
	usage, err := captureProjectCLIStderr(func() error {
		issueOpsUsage()
		return nil
	})
	if err != nil {
		t.Fatalf("capture usage: %v", err)
	}
	wantFragments := []string{
		"issueops domain-review record",
		"issueops ai-slop-clean record",
		"issueops regress",
		"issueops feedback resolve",
		"issueops decision add",
		"issueops record-routing",
		"issueops routing-score",
		"issueops remote-score",
		"compatibility-review",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(usage, fragment) {
			t.Errorf("usage text missing %q\n%s", fragment, usage)
		}
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
