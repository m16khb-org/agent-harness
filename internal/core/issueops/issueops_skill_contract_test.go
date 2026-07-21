package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssueOpsSkillKeepsCoreWorkflowContract(t *testing.T) {
	skill := readIssueOpsContractFile(t, "skills", "issueops", "SKILL.md")
	for _, want := range []string{
		"problem", "grill", "issue", "plan", "compatibility-review", "implement",
		"ai-slop-clean", "feedback", "pr", "cleanup", "RED→GREEN→SURFACE→CLEAN",
		"agent-harness issueops intent record", "agent-harness issueops design review",
		"agent-harness issueops worktree prepare --id \"$ISSUEOPS_ID\" --orchestrator auto --json",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("IssueOps skill missing current workflow contract %q", want)
		}
	}
}

func TestIssueOpsOwnershipDocumentationHasOneCurrentContract(t *testing.T) {
	documents := map[string]string{
		"skill":        readIssueOpsContractFile(t, "skills", "issueops", "SKILL.md"),
		"handoff":      readIssueOpsContractFile(t, "skills", "issueops", "references", "orca-handoff.md"),
		"cleanup":      readIssueOpsContractFile(t, "skills", "issueops", "references", "cleanup-state.md"),
		"worktree":     readIssueOpsContractFile(t, "skills", "issueops", "references", "worktree-context.md"),
		"workflow":     readIssueOpsContractFile(t, ".agent-harness", "AGENT_WORKFLOW.md"),
		"operations":   readIssueOpsContractFile(t, ".agent-harness", "OPERATIONS.md"),
		"architecture": readIssueOpsContractFile(t, ".agent-harness", "ARCHITECTURE.md"),
	}
	all := joinIssueOpsContractDocuments(documents)
	for _, want := range []string{
		"ownership_dispatching", "ownership_dispatched", "owner_orienting", "owner_active",
		"cleanup_pending_human_decision", "cleanup_executing", "closed",
		"canonical worker root", "exact lifecycle ID", "source main worktree remains available",
		"prep-only", "handoff complete", "fresh authenticated", "human",
	} {
		if !strings.Contains(all, want) {
			t.Fatalf("current ownership contract missing %q", want)
		}
	}
	for _, removed := range removedIssueOpsHandoffTerms() {
		for name, document := range documents {
			if strings.Contains(strings.ToLower(document), removed) {
				t.Fatalf("%s retains removed handoff contract term %q", name, removed)
			}
		}
	}
}

func TestIssueOpsOwnershipDocumentationPreservesParallelIndependence(t *testing.T) {
	all := strings.ToLower(joinIssueOpsContractDocuments(map[string]string{
		"handoff":  readIssueOpsContractFile(t, "skills", "issueops", "references", "orca-handoff.md"),
		"worktree": readIssueOpsContractFile(t, "skills", "issueops", "references", "worktree-context.md"),
		"workflow": readIssueOpsContractFile(t, ".agent-harness", "AGENT_WORKFLOW.md"),
	}))
	for _, want := range []string{
		"exact lifecycle id",
		"before source-wide inference",
		"unrelated active cycles",
		"linked worktree",
		"literal `issueops start --repo <exact-source>`",
	} {
		if !strings.Contains(all, want) {
			t.Fatalf("parallel ownership documentation missing %q", want)
		}
	}
}

func TestIssueOpsCurrentSurfacesDoNotNameRemovedCommands(t *testing.T) {
	for _, parts := range [][]string{
		{"internal", "adapter", "cli", "usage.go"},
		{"cmd", "harness", "issueopscli", "issueops_handoff_cli.go"},
		{"internal", "adapter", "mcp", "issueops_catalog.go"},
		{"internal", "adapter", "mcp", "issueops_lifecycle_catalog.go"},
	} {
		content := readIssueOpsContractFile(t, parts...)
		for _, removed := range removedIssueOpsHandoffTerms() {
			if strings.Contains(strings.ToLower(content), removed) {
				t.Fatalf("%s retains removed handoff surface %q", filepath.Join(parts...), removed)
			}
		}
	}
}

func removedIssueOpsHandoffTerms() []string {
	return []string{
		"protocol-v1", "protocol-v2", "protocol v1", "protocol v2",
		"handoff finish", "handoff accept", "approve-cleanup", "record-cleanup",
		"codex-hooks-list", "migrate-legacy", "approve_legacy_coordinator_seal",
	}
}

func joinIssueOpsContractDocuments(documents map[string]string) string {
	parts := make([]string, 0, len(documents))
	for _, document := range documents {
		parts = append(parts, document)
	}
	return strings.Join(parts, "\n")
}

func readIssueOpsContractFile(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{"..", "..", ".."}, parts...)...)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
