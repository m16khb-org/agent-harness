package commandparse

import "testing"

func TestFlatIssueOpsCommandAliasesPreserveLifecycleFlags(t *testing.T) {
	for _, executable := range []string{"issueops", "io", "bin/issueops", "./bin/issueops", "bin/io", "./bin/io"} {
		t.Run(executable, func(t *testing.T) {
			parsed, ok := ParseExactIssueOpsCommand(executable + " execution release --id io-example --generation 1 --host codex --session-id session --cwd '/repo with space'")
			if !ok || parsed.Path != "execution release" {
				t.Fatalf("flat command was not recognized: %#v, %v", parsed, ok)
			}
			values, booleans, repeatable, supported := IssueOpsCommandSpec(parsed.Path)
			flags, valid := ExactFlags(parsed, values, booleans, repeatable)
			if !supported || !valid || len(flags["--cwd"]) != 1 || flags["--cwd"][0] != "/repo with space" {
				t.Fatalf("flat command lost its flags: %#v", flags)
			}
		})
	}
	for _, command := range []string{"issueops execution release; echo unsafe", "io status --id $(echo unsafe)", "/tmp/issueops status --id io-example"} {
		if _, ok := ParseExactIssueOpsCommand(command); ok {
			t.Fatalf("unsafe or unprovenanced command was accepted: %q", command)
		}
	}
}
