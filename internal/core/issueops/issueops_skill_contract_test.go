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
		"agent-harness issueops execution prepare --id \"$ISSUEOPS_ID\" --mode auto",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("IssueOps skill missing current workflow contract %q", want)
		}
	}
}

func TestIssueOpsExecutionDocumentationHasOneCurrentContract(t *testing.T) {
	documents := map[string]string{
		"skill":        readIssueOpsContractFile(t, "skills", "issueops", "SKILL.md"),
		"execution":    readIssueOpsContractFile(t, "skills", "issueops", "references", "execution.md"),
		"cleanup":      readIssueOpsContractFile(t, "skills", "issueops", "references", "cleanup-state.md"),
		"worktree":     readIssueOpsContractFile(t, "skills", "issueops", "references", "worktree-context.md"),
		"start":        readIssueOpsContractFile(t, "skills", "issueops", "references", "operational-start.md"),
		"workflow":     readIssueOpsContractFile(t, ".agent-harness", "AGENT_WORKFLOW.md"),
		"operations":   readIssueOpsContractFile(t, ".agent-harness", "OPERATIONS.md"),
		"architecture": readIssueOpsContractFile(t, ".agent-harness", "ARCHITECTURE.md"),
	}
	all := joinIssueOpsContractDocuments(documents)
	for _, want := range []string{
		"one `Execution`", "canonical worktree", "exact lifecycle ID",
		"source main worktree remains available", "direct", "orca",
		"issueops execution prepare", "--mode auto", "issueops execution status",
		"issueops execution claim", "--claim-token-file", "--issue-body-sha256",
		"--context-packet-sha256", "issueops execution release",
		"issueops execution replace", "issueops execution reconcile",
		"issueops execution complete",
	} {
		if !strings.Contains(all, want) {
			t.Fatalf("current execution v1 contract missing %q", want)
		}
	}
	for _, removed := range removedIssueOpsExecutionTerms() {
		for name, document := range documents {
			if strings.Contains(strings.ToLower(document), removed) {
				t.Fatalf("%s retains removed execution contract term %q", name, removed)
			}
		}
	}
}

func TestIssueOpsExecutionDocumentationPreservesParallelIndependence(t *testing.T) {
	all := strings.ToLower(joinIssueOpsContractDocuments(map[string]string{
		"execution": readIssueOpsContractFile(t, "skills", "issueops", "references", "execution.md"),
		"worktree":  readIssueOpsContractFile(t, "skills", "issueops", "references", "worktree-context.md"),
		"workflow":  readIssueOpsContractFile(t, ".agent-harness", "AGENT_WORKFLOW.md"),
	}))
	for _, want := range []string{
		"exact lifecycle id",
		"canonical worktree",
		"one active execution per record",
		"unrelated cycles",
		"source main worktree remains available",
	} {
		if !strings.Contains(all, want) {
			t.Fatalf("parallel execution documentation missing %q", want)
		}
	}
}

func TestIssueOpsCurrentSurfacesDoNotNameRemovedCommands(t *testing.T) {
	for _, parts := range [][]string{
		{"internal", "adapter", "cli", "usage.go"},
		{"cmd", "harness", "issueopscli", "issueops_cli_support.go"},
		{"internal", "core", "commandparse", "issueops.go"},
		{"internal", "adapter", "mcp", "issueops_catalog.go"},
	} {
		content := readIssueOpsContractFile(t, parts...)
		for _, removed := range removedIssueOpsCurrentCommandTerms() {
			if strings.Contains(strings.ToLower(content), removed) {
				t.Fatalf("%s retains removed handoff surface %q", filepath.Join(parts...), removed)
			}
		}
	}
}

func removedIssueOpsCurrentCommandTerms() []string {
	return []string{
		"migrate-v9", "execution decide", "worktree prepare", "handoff start",
		"handoff claim", "handoff complete", "force-release", "resume --repo",
		"issueops heartbeat", "reconcile-create", "prepare-worktree-tools",
	}
}

func removedIssueOpsExecutionTerms() []string {
	return append(removedIssueOpsCurrentCommandTerms(),
		"execution_handoff", "ownership_epoch", "ownership_dispatch", "owner_orienting",
		"owner_active", "cleanup_pending_human_decision", "cleanup_executing",
		"--orchestrator", "resolved_mode", "prep-only", "issueops_record_execution_decision",
		"issueops_record_compatibility_review", "issueops_regress_for_replan",
	)
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
