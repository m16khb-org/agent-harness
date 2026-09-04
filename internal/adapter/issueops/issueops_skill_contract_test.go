package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 라우터는 phase enum과 공통 규칙을 소유하고, 단계 명령은 단계 스킬이 소유한다.
// 두 축을 각각 고정해 명령이 라우터로 되돌아오는 것과 스킬에서 사라지는 것을
// 함께 막는다.
func TestIssueOpsSkillKeepsCoreWorkflowContract(t *testing.T) {
	skill := readIssueOpsContractFile(t, "skills", "issueops", "SKILL.md")
	for _, want := range []string{
		"problem", "grill", "issue", "plan", "compatibility-review", "implement",
		"ai-slop-clean", "feedback", "pr", "cleanup", "RED→GREEN→SURFACE→CLEAN",
		"agent-harness issueops next --json",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("IssueOps router missing current workflow contract %q", want)
		}
	}
	stages := map[string]string{
		"agent-harness issueops intent record":                                       readIssueOpsContractFile(t, "skills", "issueops-create-issue", "SKILL.md"),
		"agent-harness issueops design review":                                       readIssueOpsContractFile(t, "skills", "issueops-plan", "SKILL.md"),
		"agent-harness issueops execution prepare --id \"$ISSUEOPS_ID\" --mode auto": readIssueOpsContractFile(t, "skills", "issueops-plan", "SKILL.md"),
	}
	for want, document := range stages {
		if !strings.Contains(document, want) {
			t.Fatalf("stage skill missing owned command %q", want)
		}
	}
}

func TestIssueOpsExecutionDocumentationHasOneCurrentContract(t *testing.T) {
	documents := map[string]string{
		"skill":        readIssueOpsContractFile(t, "skills", "issueops", "SKILL.md"),
		"execution":    readIssueOpsContractFile(t, "skills", "issueops", "references", "execution.md"),
		"cleanup":      readIssueOpsContractFile(t, "skills", "issueops", "references", "cleanup-state.md"),
		"plan":         readIssueOpsContractFile(t, "skills", "issueops-plan", "SKILL.md"),
		"providers":    readIssueOpsContractFile(t, ".agent-harness", "operations", "guides", "issueops-providers.md"),
		"workflow":     readIssueOpsContractFile(t, ".agent-harness", "AGENT_WORKFLOW.md"),
		"operations":   readIssueOpsContractFile(t, ".agent-harness", "OPERATIONS.md"),
		"architecture": readIssueOpsContractFile(t, ".agent-harness", "ARCHITECTURE.md"),
	}
	all := joinIssueOpsContractDocuments(documents)
	for _, want := range []string{
		"one `Execution`", "canonical worktree", "exact lifecycle ID",
		"source main worktree remains available", "direct", "orca",
		"issueops execution prepare", "--mode auto", "issueops execution status",
		"issueops execution claim", "--claim-current-token", "--issue-body-sha256",
		"--context-packet-sha256", "issueops execution release",
		"issueops execution replace", "issueops execution reconcile",
		"issueops execution complete", "requested_mode", "resolved_mode",
		"readiness fingerprint",
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
	if count := strings.Count(documents["execution"], "agent-harness issueops execution prepare \\"); count != 1 {
		t.Fatalf("execution reference must show one preview command and delegate confirm to next_command, count=%d", count)
	}
}

func TestIssueOpsDocumentationPreservesGitHubOrcaBranchOrdering(t *testing.T) {
	operationsIndex := readIssueOpsContractFile(t, ".agent-harness", "OPERATIONS.md")
	if !strings.Contains(operationsIndex, "operations/guides/issueops-providers.md") {
		t.Fatal("OPERATIONS.md must route provider ordering to operations/guides/issueops-providers.md")
	}
	// 순서 문자열의 소유자는 둘이다: 실행 계약 레퍼런스와 provider 운영 가이드.
	// 단계 스킬은 그 둘을 링크하고 순서를 복사하지 않는다.
	documents := map[string]string{
		"execution":  readIssueOpsContractFile(t, "skills", "issueops", "references", "execution.md"),
		"operations": readIssueOpsContractFile(t, ".agent-harness", "operations", "guides", "issueops-providers.md"),
	}
	const want = "`branch prepare` (base SHA only) → `artifact stage --name plan` → `execution prepare --mode orca` → GraphQL `createLinkedBranch` with `oid=sealed base SHA` → `branch prepare --link-verified`"
	for name, document := range documents {
		if !strings.Contains(document, want) {
			t.Errorf("%s missing GitHub Orca branch ordering %q", name, want)
		}
		if strings.Contains(document, "gh issue develop --name") {
			t.Errorf("%s reintroduces the superseded GitHub Orca branch creation command", name)
		}
	}
}

func TestIssueOpsExecutionDocumentationPreservesParallelIndependence(t *testing.T) {
	all := strings.ToLower(joinIssueOpsContractDocuments(map[string]string{
		"execution": readIssueOpsContractFile(t, "skills", "issueops", "references", "execution.md"),
		"router":    readIssueOpsContractFile(t, "skills", "issueops", "SKILL.md"),
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

func TestIssueOpsOrchestrationBindsOmoAgentsToCanonicalWorktrees(t *testing.T) {
	all := strings.ToLower(joinIssueOpsContractDocuments(map[string]string{
		"orchestration": readIssueOpsContractFile(t, "skills", "issueops", "references", "orchestration.md"),
		"host-testing":  readIssueOpsContractFile(t, ".agent-harness", "testing", "cli-mcp-and-hosts.md"),
	}))
	for _, want := range []string{
		"team_create.members[].worktreepath",
		"resident child process `cwd`",
		"plain `task`",
		"pi_session_id",
		"issueops execution whoami --json",
		"branded `omo -p`",
	} {
		if !strings.Contains(all, want) {
			t.Fatalf("Omo worktree orchestration contract missing %q", want)
		}
	}
}

func TestIssueOpsCurrentSurfacesDoNotNameRemovedCommands(t *testing.T) {
	for _, parts := range [][]string{
		{"internal", "domain", "cli", "usage.go"},
		{"cmd", "harness", "issueopscli", "issueops_cli_support.go"},
		{"internal", "domain", "commandparse", "issueops.go"},
		{"internal", "domain", "mcp", "issueops_catalog.go"},
	} {
		content := readIssueOpsContractFile(t, parts...)
		for _, removed := range removedIssueOpsCurrentCommandTerms() {
			if strings.Contains(strings.ToLower(content), removed) {
				t.Fatalf("%s retains removed handoff surface %q", filepath.Join(parts...), removed)
			}
		}
	}
}

// .agent-harness/SUB_AGENT_PATTERNS.md is routed from the issueops skill as the
// sub-agent decision contract, so it must not resurrect removed decision
// commands or MCP tools either.
func TestSubAgentPatternsDocDoesNotNameRemovedIssueOpsSurfaces(t *testing.T) {
	content := strings.ToLower(readIssueOpsContractFile(t, ".agent-harness", "SUB_AGENT_PATTERNS.md"))
	for _, removed := range removedIssueOpsExecutionTerms() {
		if strings.Contains(content, removed) {
			t.Fatalf(".agent-harness/SUB_AGENT_PATTERNS.md retains removed surface %q", removed)
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
		"--orchestrator", "prep-only", "issueops_record_execution_decision",
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
