package issueopscli

import (
	"strings"
	"testing"
)

func TestIssueOpsHandoffExposesOnlyCurrentActions(t *testing.T) {
	for _, current := range []string{
		"handoff start", "handoff claim", "handoff acknowledge-context",
		"handoff publish", "handoff complete", "handoff cleanup-preview",
		"handoff cleanup-approve", "handoff cleanup-record", "handoff recover",
	} {
		if !strings.Contains(issueOpsHandoffUsage, current) {
			t.Fatalf("current handoff usage is missing %q", current)
		}
	}
	for _, removed := range []string{"handoff finish", "handoff accept", "retry", "approve-cleanup", "record-cleanup", "protocol-v1", "protocol-v2"} {
		if strings.Contains(issueOpsHandoffUsage, removed) {
			t.Fatalf("removed handoff action or protocol remains in usage: %q", removed)
		}
	}
	for _, removed := range []string{"finish", "accept"} {
		if err := runIssueOpsHandoff([]string{removed}); err == nil {
			t.Fatalf("removed handoff subcommand %q was accepted", removed)
		}
	}
}
