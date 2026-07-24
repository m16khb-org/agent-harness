package issueopscli

import (
	"flag"
	"strings"
	"testing"
)

// TestRepeatedFlagRoundTripsRepeatedValues는 표준 반복 가능 문자열 flag를 실제
// FlagSet으로 검증한다. --flag가 등장할 때마다 값 하나를 append하고, String()은
// 모인 값들을 단일 "," 구분자로 잇는다.
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

// TestIssueOpsUsageListsNewlyAddedSubcommands는 usage 텍스트를 docs-drift
// 회귀로부터 지킨다. 감사에서 누락으로 드러난, issueOpsSubcommands에 등록된 모든
// subcommand는 issueOpsUsage()에 나타나야 한다.
func TestIssueOpsUsageListsNewlyAddedSubcommands(t *testing.T) {
	usage, err := captureProjectCLIStderr(t, func() error {
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
		"issueops execution prepare --id ID --mode auto|direct|orca",
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
