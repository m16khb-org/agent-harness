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
		"./cmd/harness/hookcli/hookinput",
		"bun scripts/smoke-gjc-native-hook.ts",
		"Do not use a literal `--host gjc` grep",
		"`status`, `dispatch`, `worker_done`, `merge_ready`, `escalation`, `handoff`, `decision_gate`, and `heartbeat`",
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
		"`closed/accepted` is terminal and cannot be retried",
		"Only `closed/worker_failed` and `closed/cancelled` may start a new attempt",
		"claimed cancel is fail-closed without explicit stale or force evidence",
		"unresolved pending operation journal survives cancel",
		"context source fingerprint",
		"Turing report must exist inside the canonical worker root",
		"worker worktree must be clean",
		"attempt_base_head",
		"clean exact branch and HEAD checkpoint",
		"worktree removal is not terminal cleanup evidence",
		"exact spawned handle and PTY",
		"connected=false or absent from terminal list",
		"nested shells",
		"representative mutation family",
		"explicit payload and CLI hosts conflict",
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
		"./cmd/harness/hookcli/hookinput",
		"bun scripts/smoke-gjc-native-hook.ts",
		"Do not use a literal `--host gjc` grep",
		"`status`, `dispatch`, `worker_done`, `merge_ready`, `escalation`, `handoff`, `decision_gate`, and `heartbeat`",
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
		"`closed/accepted` is terminal and cannot be retried",
		"Only `closed/worker_failed` and `closed/cancelled` may start a new attempt",
		"context source fingerprint",
		"Turing report must exist inside the canonical worker root",
		"worker worktree must be clean",
		"attempt_base_head",
		"clean exact branch and HEAD checkpoint",
		"worktree removal is not terminal cleanup evidence",
		"exact spawned handle and PTY",
		"connected=false or absent from terminal list",
		"nested shells",
		"representative mutation family",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("Turing supervised handoff contract missing phrase %q", want)
		}
	}
}

func TestSupervisedHandoffSkillsPinCorrectiveOperationalRecipes(t *testing.T) {
	issueOps := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	turing := readTuringSkillForTest(t)
	for name, body := range map[string]string{"IssueOps": issueOps, "Turing": turing} {
		for _, want := range []string{
			"source checkout is observation-only",
			"Tests, builds, formatting, installation, and generation run only in the claimed worker root",
			"orca terminal send --terminal <handle> --text <payload> --enter --json",
			"POSIX single-quote",
			"JS template interpolation",
			"accepted FinalHead",
			"refs/heads/<branch>",
			"explicit head/source and base/target flags",
			"draft PR/MR",
			"eval and source",
			"zsh equals expansion",
			"unquoted process substitution",
			"pathname expansion",
			"Turing report path is a safe relative path",
			"leaf symlink",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s supervised handoff recipe missing %q", name, want)
			}
		}
	}
	if strings.Contains(turing, "Reasonix") || !strings.Contains(turing, "Codex, Claude, and GJC") {
		t.Fatal("Turing host portability contract must use Codex, Claude, and GJC only")
	}
}

func TestIssueOpsPublishRecipeAvoidsShellCommandSubstitution(t *testing.T) {
	body := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	sectionStart := strings.Index(body, "## Coordinator Publish")
	if sectionStart < 0 {
		t.Fatal("IssueOps Orca reference must have a Coordinator Publish section")
	}
	section := body[sectionStart:]
	fenceStart := strings.Index(section, "```bash\n")
	if fenceStart < 0 {
		t.Fatal("Coordinator Publish section must contain an executable bash recipe")
	}
	recipe := section[fenceStart+len("```bash\n"):]
	if fenceEnd := strings.Index(recipe, "```"); fenceEnd >= 0 {
		recipe = recipe[:fenceEnd]
	}
	if strings.Contains(recipe, "$(") || strings.ContainsRune(recipe, '`') {
		t.Fatalf("publish recipe must not use shell command substitution: %s", recipe)
	}
	for _, want := range []string{"git rev-parse --verify refs/heads/<branch>", "git push <remote> <branch>"} {
		if !strings.Contains(recipe, want) {
			t.Fatalf("publish recipe missing %q", want)
		}
	}
}

func TestIssueOpsSupervisedPlanMustDescribeCurrentCycle(t *testing.T) {
	body := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	for _, want := range []string{
		"current issue and cycle intent",
		"acceptance criteria",
		"exact bounded worker scope",
		"Never link an unrelated legacy plan",
		"coordinator plan commit",
		"attempt base head",
		"source coordinator root",
		"feature-worktree session must not steer a child plan",
		"raw terminal steering",
		"persisted worker terminal handle",
		"uniquely matching persisted worker terminal handle",
		"issueops handoff start",
		"target hook",
		"ASCII C0",
		"DEL",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("IssueOps supervised plan contract missing %q", want)
		}
	}
}

func TestTuringSupervisedPlanMustDescribeCurrentCycle(t *testing.T) {
	body := readTuringSkillForTest(t)
	for _, want := range []string{
		"current issue and cycle intent",
		"acceptance criteria",
		"exact bounded worker scope",
		"Never link an unrelated legacy plan",
		"coordinator plan commit",
		"attempt base head",
		"source coordinator root",
		"feature-worktree session must not steer a child plan",
		"raw terminal steering",
		"persisted worker terminal handle",
		"uniquely matching persisted worker terminal handle",
		"issueops handoff start",
		"target hook",
		"ASCII C0",
		"DEL",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("Turing supervised plan contract missing %q", want)
		}
	}
}

func TestSelfVerifySkillKeepsGenericLLMEvalContractWithoutOrcaRecipes(t *testing.T) {
	body := readSelfVerifySkillForTest(t)
	for _, want := range []string{"read-only evaluator prompt", "No Z.AI request is sent", "explicit `--llm-eval=false`"} {
		if !strings.Contains(body, want) {
			t.Fatalf("self-verify generic llm-eval contract missing %q", want)
		}
	}
	for _, forbidden := range []string{"For Orca handoff changes", "smoke-gjc-native-hook.ts", "--host gjc"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("self-verify skill must not carry IssueOps/Turing-only recipe %q", forbidden)
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

func readSelfVerifySkillForTest(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "skills", "self-verify", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
