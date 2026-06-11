package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssueOpsSkillRoutesPhasesToAgentHarnessFeatures(t *testing.T) {
	skill := readIssueOpsSkillForTest(t)
	for _, want := range []string{
		"Agent-Harness Phase Assist Map",
		"deep-interview",
		"problem",
		"grill",
		"issue",
		"issue-preflight",
		"PROMPT.md",
		"ideal issue prompt",
		"raw user request",
		"agent-harness issueops intent record",
		"agent-harness issueops design review",
		"design review checked alternatives and risks",
		"there is no approve-only merge step",
		"main agent's judgment",
		"approved and has no open questions",
		"ambiguity ledger",
		"plan",
		"implement",
		"ai-slop-clean",
		"feedback",
		"pr",
		"cleanup",
		"von-neumann",
		"berners-lee",
		"codd",
		"dijkstra",
		"hopper",
		"turing",
		"shannon",
		"torvalds",
		"atomic-commit-push",
		"Explore Before Asking",
		"RED→GREEN→SURFACE→CLEAN",
		"Manual-QA across 4 channels",
		"dependency matrix",
		"parallel execution waves",
		"Hyperlink Contract",
		"1NF→BCNF",
		"O(n²)→O(n log n)",
		"7-step Hopper Method",
		"SNR",
		"Devil's advocate",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("IssueOps skill missing agent-harness phase routing contract %q", want)
		}
	}
}

func TestIssueOpsSkillRequiresQualityUpgradeContracts(t *testing.T) {
	skill := readIssueOpsSkillForTest(t)
	refs := readIssueOpsReferenceForTest(t, "remote-issue.md") + "\n" +
		readIssueOpsReferenceForTest(t, "review-feedback.md") + "\n" +
		readIssueOpsReferenceForTest(t, "evidence-contract.md")

	for _, want := range []string{
		"threshold-based label decision",
		"selected labels, rejected labels, and manual override reason",
		"Large Issue Breakdown Gate",
		"provider-native child work items",
		"draft issue completion record",
		"review-agent feedback",
		"Kodus",
		"Gemini Code Assist",
		"resolveReviewThread",
		"resolved=true",
	} {
		if !strings.Contains(skill+"\n"+refs, want) {
			t.Fatalf("IssueOps skill/reference contract missing quality upgrade phrase %q", want)
		}
	}
}

func TestIssueOpsSkillDocumentsReadinessGateKeys(t *testing.T) {
	skill := readIssueOpsSkillForTest(t)
	for _, want := range []string{
		"intent_contract",
		"branch_prepare",
		"branch_link_verified",
		"worktree_path",
		"worktree_exists",
		"design_review",
		"design_approval",
		"design_review_evidence",
		"refactor_plan",
		"alternatives",
		"risks",
		"design_open_questions",
		"plan_path",
		"plan_exists",
		"plan_in_worktree",
		"ai_slop_clean",
		"contract_feedback_issue_update",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("IssueOps skill must document readiness gate key %q", want)
		}
	}
}

// Asserts the pioneer-targeted benchmark fixtures exist with the method-skip
// critical rule. NOTE: like the other contract tests in this file, this
// verifies file/text PRESENCE only — it does not prove runtime behavior; the
// behavioral coverage lives in the benchmark package tests.
func TestIssueOpsPioneerFixturesCarryMethodSkipRule(t *testing.T) {
	for _, name := range []string{"pioneer-dijkstra", "pioneer-codd", "pioneer-hopper", "pioneer-shannon"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "issueops", "fixtures", name+".json"))
		if err != nil {
			t.Fatalf("pioneer fixture %s missing: %v", name, err)
		}
		body := string(b)
		if !strings.Contains(body, "skips pioneer method") {
			t.Fatalf("fixture %s must carry the \"skips pioneer method\" critical rule", name)
		}
		if !strings.Contains(body, "\"pioneer_skill_target\"") {
			t.Fatalf("fixture %s must set pioneer_skill_target", name)
		}
	}
}

func readIssueOpsSkillForTest(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "skills", "issueops", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func readIssueOpsReferenceForTest(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "skills", "issueops", "references", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
