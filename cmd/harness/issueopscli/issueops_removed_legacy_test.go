package issueopscli

import (
	"strings"
	"testing"
)

func TestIssueOpsRejectsRemovedResetCommand(t *testing.T) {
	command := strings.Join([]string{"reset", "legacy"}, "-")
	err := runIssueOps([]string{command})
	if err == nil || !strings.Contains(err.Error(), "unknown issueops subcommand") {
		t.Fatalf("removed command error=%v", err)
	}
	if _, ok := issueOpsSubcommands[command]; ok {
		t.Fatalf("removed command remains registered")
	}
}
