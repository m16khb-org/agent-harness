package commandparse

import "testing"

func TestParseExactIssueOpsDesignReview(t *testing.T) {
	commandText := "issueops design review" +
		" --id io-1 --problem-summary problem --proposed-design design --refactor-plan boundary" +
		" --alternative one --alternative two --risk risk-one --risk risk-two" +
		" --verification check-one --verification check-two --open-question question-one --open-question question-two" +
		" --host codex --session-id session-1 --agent-id agent-1 --cwd /repo --approved --json"
	command, ok := ParseExactIssueOpsCommand(commandText)
	if !ok || command.Path != "design review" {
		t.Fatalf("design review did not parse as an exact two-word command: %#v ok=%v", command, ok)
	}

	values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatal("design review has no exact flag spec")
	}
	flags, ok := ExactFlags(command, values, booleans, repeatable)
	if !ok {
		t.Fatal("design review's existing CLI flags were rejected")
	}
	for _, name := range []string{
		"--id", "--problem-summary", "--proposed-design", "--refactor-plan",
		"--host", "--session-id", "--agent-id", "--cwd",
	} {
		if len(flags[name]) != 1 {
			t.Fatalf("scalar flag %s = %#v", name, flags[name])
		}
	}
	for _, name := range []string{"--alternative", "--risk", "--verification", "--open-question"} {
		if len(flags[name]) != 2 {
			t.Fatalf("repeatable flag %s = %#v", name, flags[name])
		}
	}
	if len(flags["--approved"]) != 1 || len(flags["--json"]) != 1 {
		t.Fatalf("boolean flags were not preserved: %#v", flags)
	}

	unknown, _ := ParseExactIssueOpsCommand(commandText + " --unknown value")
	if got, accepted := ExactFlags(unknown, values, booleans, repeatable); accepted || got != nil {
		t.Fatalf("unknown design review flag was accepted: %#v", got)
	}

	nearMiss, parsed := ParseExactIssueOpsCommand("issueops design approve --id io-1")
	if parsed {
		if _, _, _, accepted := IssueOpsCommandSpec(nearMiss.Path); accepted {
			t.Fatalf("malformed design path was classified: %#v", nearMiss)
		}
	}
}
