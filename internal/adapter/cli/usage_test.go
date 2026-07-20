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
	for _, want := range []string{"policy audit", "contract schema", "quality inspect", "worker enqueue"} {
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

func TestUsageIncludesOwnershipTransferHandoffActions(t *testing.T) {
	usage := Usage("test")
	for _, action := range []string{"complete", "cleanup-preview", "cleanup-approve", "cleanup-record"} {
		if !strings.Contains(usage, action) {
			t.Fatalf("usage missing ownership-transfer handoff action %q\n%s", action, usage)
		}
	}
}
