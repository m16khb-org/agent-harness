package cli

import (
	"strings"
	"testing"
)

func TestUsageIncludesCommandCatalog(t *testing.T) {
	usage := Usage("test")
	for _, command := range Commands() {
		if !strings.Contains(usage, "harness "+command.Name) && command.Name != "version" {
			t.Fatalf("usage does not mention command %q\n%s", command.Name, usage)
		}
	}
	for _, want := range []string{"policy audit", "contract schema", "quality inspect", "worker enqueue", "issueops devils-advocate review"} {
		if !strings.Contains(usage, want) {
			t.Fatalf("usage missing %q", want)
		}
	}
}
func TestUsageIncludesUpdateCommand(t *testing.T) {
	usage := Usage("test")
	if !strings.Contains(usage, "agent-harness update") {
		t.Fatalf("usage missing update command\n%s", usage)
	}
	found := false
	for _, command := range Commands() {
		if command.Name == "update" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("command catalog missing update")
	}
}

func TestUsageOmitsRetiredSelfVerifyModes(t *testing.T) {
	for line := range strings.SplitSeq(Usage("test"), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "agent-harness self-verify ") {
			continue
		}
		for _, retired := range []string{"--full", "--iterations"} {
			if strings.Contains(line, retired) {
				t.Fatalf("usage still advertises retired self-verify mode %q", retired)
			}
		}
		return
	}
	t.Fatal("usage missing self-verify command")
}

func TestUsageIncludesIssueOpsExecutionActions(t *testing.T) {
	usage := Usage("test")
	for _, action := range []string{
		"execution prepare",
		"execution status",
		"execution claim",
		"execution release",
		"execution replace",
		"execution resume",
		"execution reconcile",
		"execution complete",
	} {
		if !strings.Contains(usage, action) {
			t.Fatalf("usage missing IssueOps v1 action %q\n%s", action, usage)
		}
	}
}

func TestUsageDocumentsExecutionReplaceCompletionGeneration(t *testing.T) {
	usage := Usage("test")
	if !strings.Contains(usage, "execution replace") || !strings.Contains(usage, "--completion-generation N") {
		t.Fatalf("usage must document the completion-bearing reseed flag\n%s", usage)
	}
}

func TestUsageOmitsRetiredStateAndIssueOpsMigration(t *testing.T) {
	usage := Usage("test")
	for _, retired := range []string{
		strings.Join([]string{"state", "migrate"}, " "),
		strings.Join([]string{"reset", "legacy"}, "-"),
	} {
		if strings.Contains(usage, retired) {
			t.Fatalf("usage still advertises retired surface %q", retired)
		}
	}
}

func TestUsageOmitsRetiredPoolCommand(t *testing.T) {
	retiredCommand := strings.Join([]string{"work", "pool"}, "")
	for _, command := range Commands() {
		if command.Name == retiredCommand {
			t.Fatalf("%s command must be removed", retiredCommand)
		}
	}
	if strings.Contains(Usage("test"), "agent-harness "+retiredCommand) {
		t.Fatalf("usage must not advertise %s", retiredCommand)
	}
}
