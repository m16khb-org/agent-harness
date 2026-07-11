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

func TestIssueOpsSkillDocumentsLargeIssueBreakdownGateForBothProviders(t *testing.T) {
	skill := readIssueOpsSkillForTest(t)
	remoteIssue := readIssueOpsReferenceForTest(t, "remote-issue.md")
	body := skill + "\n" + remoteIssue

	for _, want := range []string{
		"Issue Contract 이후, Plan 이전",
		"Before entering the IssueOps `plan` phase",
		"default decision is no split",
		"one issue would be unsafe",
		"explicitly requested for collaboration",
		"Supporting signals are not sufficient by themselves",
		"분리 결정: split",
		"분리 결정: no split",
		"GitHub: create sub-issues",
		"GitLab: create child `Task` work items",
		"agent-harness issueops remote create-child",
		"gh api",
		"glab api",
		"provider-native hierarchy",
		"parent body updated",
		"parallelizable",
		"sequential",
		"[p]",
		"[s]",
		"prerequisites",
		"execution waves",
		"workItemCreate",
		"workItemHierarchyAddChildrenItems",
		"REST Issues API `issue_type=task`",
		"GraphQL failure is a blocker",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("IssueOps large issue breakdown gate must document provider-neutral contract phrase %q", want)
		}
	}
}

func TestIssueOpsSkillDocumentsReadinessGateKeys(t *testing.T) {
	skill := readIssueOpsSkillForTest(t)
	for _, want := range []string{
		"intent_contract",
		"plan_prep_decisions",
		"plan_prep_related_issues",
		"plan_prep_web_research",
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

func TestIssueOpsSkillDocumentsOrchestrationContracts(t *testing.T) {
	skill := readIssueOpsSkillForTest(t)
	reference := readIssueOpsReferenceForTest(t, "orchestration.md")
	body := skill + "\n" + reference

	for _, want := range []string{
		"Delegated Child Cycles",
		"Worker Pool",
		"issueops child start",
		"issueops child status",
		"issueops child accept",
		"issueops child reject",
		"issueops child drop",
		"workpool claim",
		"workpool submit",
		"workpool accept",
		"child_incomplete",
		"child_unvalidated",
		"child_rejected_unresolved",
		"pool_incomplete",
		"children_active",
		"implement phase",
		"approved reviews",
		"recorded sub-agent plan",
		"accepted",
		"rejected",
		"dropped",
		"child contract",
		"pool worker loop",
		"scope-drift stop rule",
		"validation rubric",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("IssueOps orchestration skill/reference contract missing phrase %q", want)
		}
	}
}

func TestIssueOpsSkillDocumentsOrchestrationOwnerCommands(t *testing.T) {
	skill := readIssueOpsSkillForTest(t)
	for _, want := range []string{
		"`child_incomplete` | `issueops child status`",
		"`child_unvalidated` | `issueops child accept`",
		"`child_rejected_unresolved` | `issueops child accept` or `issueops child drop`",
		"`pool_incomplete` | `workpool status`",
		"`children_active` | `issueops child status`",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("IssueOps skill must map orchestration missing key to owner command %q", want)
		}
	}
}

func TestIssueOpsSkillDocumentsOptionalOrcaHandoffContract(t *testing.T) {
	skill := readIssueOpsSkillForTest(t)
	reference := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	body := skill + "\n" + reference

	for _, want := range []string{
		"references/orca-handoff.md",
		"--orchestrator auto",
		"--orchestrator orca",
		"--orchestrator inline",
		"issueops worktree prepare",
		"issueops handoff start",
		"issueops handoff claim",
		"issueops heartbeat",
		"issueops handoff finish",
		"issueops handoff accept",
		"issueops handoff recover",
		"never retry a create operation",
		"exactly one candidate",
		"source implementation checkout is read-only",
		"go build -o",
		"self-verify requires binary/source contract parity",
		"No Z.AI request is sent",
		"explicit `--llm-eval=false`",
		"orca orchestration task-list --json",
		"orca orchestration dispatch-show --task <id> --json",
		"orca worktree show --worktree id:<id> --json",
		"orca task show",
		"--task-id",
		"installed `agent-harness`",
		"`./bin/agent-harness` exists",
		"issue #17",
		"send one escalation",
		"keep heartbeat",
		"remain mutation-free",
		"must not invoke `orca orchestration ask`",
		"duplicate ask or decision gate",
		"active Codex session may retain its previously loaded hook command",
		"Installed-file readback alone is insufficient",
		"live current-session probe is authoritative",
		"both the payload host and `--host` are empty",
		"exact nonempty session",
		"top-level `transcript_path` and `agent_transcript_path` are hook metadata",
		"tool_input paths and patch targets remain enforced",
		"full-payload probe",
		"Quoted semicolons, ampersands, and pipes in evidence values are argument data",
		"unquoted shell control operators and newlines remain blocked",
		"Omit `--agent-id` when the native agent id is empty",
		"worker stops",
		"coordinator owns PR, acceptance, and cleanup",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("IssueOps Orca handoff contract missing phrase %q", want)
		}
	}
}

func TestTuringSkillDocumentsSupervisedHandoffEvidenceContract(t *testing.T) {
	skill := readTuringSkillForTest(t)
	for _, want := range []string{
		"issueops heartbeat",
		"ORCA-01",
		"ORCA-14",
		"handoff result report",
		"cleanup receipts",
		"issueops handoff finish",
		"source implementation checkout is read-only",
		"self-verify requires binary/source contract parity",
		"No Z.AI request is sent",
		"explicit `--llm-eval=false`",
		"installed `agent-harness`",
		"`./bin/agent-harness` exists",
		"send one escalation",
		"keep heartbeat",
		"remain mutation-free",
		"must not invoke `orca orchestration ask`",
		"active Codex session may retain its previously loaded hook command",
		"Installed-file readback alone is insufficient",
		"live current-session probe is authoritative",
		"both the payload host and `--host` are empty",
		"exact nonempty session",
		"top-level `transcript_path` and `agent_transcript_path` are hook metadata",
		"tool_input paths and patch targets remain enforced",
		"full-payload probe",
		"Quoted semicolons, ampersands, and pipes in evidence values are argument data",
		"unquoted shell control operators and newlines remain blocked",
		"Omit `--agent-id` when the native agent id is empty",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("Turing supervised handoff contract missing phrase %q", want)
		}
	}
}

// Asserts the SKILL.md documents the subagent judge protocol. NOTE: text
// presence only — the no-self-approval constraint is a documented protocol,
// not enforceable at the Go layer (the file backend only sees bytes).
func TestIssueOpsSkillDocumentsSubagentJudgeProtocol(t *testing.T) {
	skill := readIssueOpsSkillForTest(t)
	for _, want := range []string{
		"fresh-context",
		"--judge file",
		"--judge none",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("IssueOps skill must document the subagent judge protocol phrase %q", want)
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

// Asserts every fixture declaring expected_routing also carries the paired
// "skips expected routing" critical rule (A5), so a recorded trace where the
// skill did not fire hard-fails a single run — parallel to "skips pioneer
// method". PRESENCE-only, like the other contract tests in this file.
func TestIssueOpsRoutingFixturesCarrySkipRule(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "testdata", "issueops", "fixtures")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		body := string(b)
		if !strings.Contains(body, "\"expected_routing\"") {
			continue
		}
		checked++
		if !strings.Contains(body, "skips expected routing") {
			t.Fatalf("fixture %s declares expected_routing but lacks the \"skips expected routing\" critical rule", entry.Name())
		}
	}
	if checked == 0 {
		t.Fatal("expected at least one fixture with expected_routing to exercise the routing-fidelity contract")
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

func readTuringSkillForTest(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "skills", "turing", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
