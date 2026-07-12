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
		"orca orchestration task-list --ready --json",
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
		"orca terminal close --terminal <resolved-worker-terminal-handle> --json",
		"optional bounded cleanup attempt",
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
		"optional bounded cleanup attempt",
		"nested shells",
		"representative mutation family",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("Turing supervised handoff contract missing phrase %q", want)
		}
	}
}

func TestTuringSkillUsesZshSafeWrapperResultVariable(t *testing.T) {
	skill := readTuringSkillForTest(t)
	for _, want := range []string{
		"zsh reserves `status` as a read-only parameter",
		"Use `rc` or `exit_code`",
		"test command verdict separately from wrapper bookkeeping errors",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("Turing zsh verification-wrapper contract missing %q", want)
		}
	}
	if strings.Contains(skill, "status=$?") {
		t.Fatal("Turing must not prescribe assignment to zsh's reserved status parameter")
	}
}

func TestTuringSkillUsesLiteralBacktickSearchPatterns(t *testing.T) {
	skill := readTuringSkillForTest(t)
	for _, want := range []string{
		"Shell arguments containing Markdown backticks must be single-quoted or passed as direct argv",
		"never place backticks inside a double-quoted shell command argument",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("Turing literal search-pattern contract missing %q", want)
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
			"never use `--unread --inject`",
			"Select the numeric `sequence` plus exact `taskId`, `dispatchId`",
			"live terminal handle is not historical mailbox identity",
			"PreToolUse blocks every other explicit message type",
			"--allow-codex-hook-trust-bypass",
			"codex app-server --stdio",
			"warnings and errors are both empty",
			"SessionStart and PreToolUse",
			"untrusted or modified",
			"--dangerously-bypass-hook-trust",
			"ContextVersion 1",
			"per-attempt attestation",
			"automatic trust probing remains issue #17",
			"second no-confirm preview",
			"reviewed context hash",
			"Final confirmed start must add `--expected-context-sha256` with the exact final attested preview hash plus `--confirm`; all delivery options remain identical.",
			"any explicit `--inject`",
			"repeat-prevention guard",
			"`[no tests to run]` is not GREEN",
			"even when `execution_handoff` is absent",
			"`gate-list` remains read-only",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s supervised handoff recipe missing %q", name, want)
			}
		}
		for _, forbidden := range []string{"--lines", "--interrupt", "orca terminal rm"} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("%s supervised handoff recipe contains forbidden terminal form %q", name, forbidden)
			}
		}
	}
	for _, want := range []string{"--coordinator-recipient", "official exact coordinator and task label lines", "exact `--dispatch-id` token"} {
		if !strings.Contains(issueOps, want) {
			t.Fatalf("IssueOps sealed dispatch context missing %q", want)
		}
	}
	for _, want := range []string{
		"runtime_refresh",
		"tabId and leafId",
		"visualLayouts[].root.tabs[].title",
		"dynamic terminal title",
		"exact-compares the journal snapshot",
		"revalidates the sealed context source and clean exact branch/HEAD",
		"Never launch a replacement",
		"uncommitted WIP",
		"orca worktree list --repo path:<exact-repo> --limit 512 --json",
		"orca terminal list --worktree id:<persisted-worktree-id> --limit 512 --json",
		"orca terminal read --terminal <recovered-current-handle> --cursor <nextCursor> --limit 1000 --json",
		"orca orchestration check --all --json | jq '.result.messages[]'",
		"top-level `.messages`",
		"caller-side Ctrl-C or host tool cancellation",
		"orca terminal stop --worktree id:<persisted-worktree-id> --json",
		"Never use `terminal rm`",
	} {
		if !strings.Contains(issueOps, want) {
			t.Fatalf("IssueOps runtime recovery recipe missing %q", want)
		}
	}
	for _, want := range []string{
		"skills/issueops/references/orca-handoff.md",
		"runtime-rollover evidence",
		"mailbox-observation receipt",
		"cleanup receipts",
	} {
		if !strings.Contains(turing, want) {
			t.Fatalf("Turing recovery evidence integration missing %q", want)
		}
	}
	for _, duplicated := range []string{
		"runtime_refresh",
		"tabId and leafId",
		"visualLayouts[].root.tabs[].title",
		"orca worktree list --repo path:<exact-repo>",
		"orca terminal list --worktree id:<persisted-worktree-id> --limit 512",
		"orca terminal read --terminal <recovered-current-handle>",
		"orca orchestration check --all --json",
		".result.messages",
		"orca terminal close --terminal <resolved-worker-terminal-handle> --json",
		"orca terminal stop --worktree id:<persisted-worktree-id>",
		"orca worktree rm --worktree id:<persisted-worktree-id>",
		"orca terminal list --worktree id:<persisted-worktree-id>",
	} {
		if strings.Contains(turing, duplicated) {
			t.Fatalf("Turing duplicates canonical IssueOps runtime recovery recipe %q", duplicated)
		}
	}
	if strings.Contains(turing, "Reasonix") || !strings.Contains(turing, "Codex, Claude, and GJC") {
		t.Fatal("Turing host portability contract must use Codex, Claude, and GJC only")
	}
}

func TestSupervisedHandoffSkillsPinObservedSoleWriterIncidents(t *testing.T) {
	issueOps := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	turing := readTuringSkillForTest(t)
	for name, body := range map[string]string{"IssueOps": issueOps, "Turing": turing} {
		for _, want := range []string{
			"completed dispatch is never a mutation lease",
			"new ready task",
			"fresh dispatch",
			"exact sole-writer attestation",
			"Never send edit instructions to a completed worker",
			"exact-worktree terminals and active orchestration tasks",
			"Any such possible writer or dispatched task blocks another writer",
			"A stable diff is not ownership evidence",
			"login shell",
			"actual host banner",
			"fresh `connected=true` and `writable=true` check",
			"One `tui-idle` sample alone is insufficient",
			"UserPromptSubmit or working state actually began",
			"send exactly one Enter",
			"Never resend the instruction body",
			"sender and recipient direction",
			"Sequence is evidence, not a lease fence",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s observed sole-writer contract missing %q", name, want)
			}
		}
		if strings.Contains(body, "task, dispatch, and sequence fence") {
			t.Fatalf("%s must not describe mailbox sequence as part of the lease fence", name)
		}
	}
}

func TestCautionsPinsDuplicateWriterInventoryRecheckIncident(t *testing.T) {
	body := readCautionsForTest(t)
	for _, want := range []string{
		"요약만 믿고",
		"exact, untruncated worktree terminal inventory",
		"connected 또는 writable한 다른 terminal이 하나라도 있으면",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("CAUTIONS.md missing duplicate-writer inventory-recheck incident: want %q", want)
		}
	}
}

func TestOrcaHandoffPinsYieldedVerificationProvenanceIncident(t *testing.T) {
	issueOps := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	cautions := readCautionsForTest(t)
	for name, body := range map[string]string{"IssueOps": issueOps, "CAUTIONS": cautions} {
		for _, want := range []string{
			"pipeline",
			"session_id",
			"write_stdin",
			"exit_code",
			"tui-idle",
			"filesystem quiescence",
			"partial package output",
			"active tool/process",
			"latest `tool_result`",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s yielded verification provenance incident missing %q", name, want)
			}
		}
	}
}

func TestSupervisedHandoffSkillsRequireCompletionFence(t *testing.T) {
	issueOps := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	turing := readTuringSkillForTest(t)
	for name, body := range map[string]string{"IssueOps": issueOps, "Turing": turing} {
		for _, want := range []string{
			"After verification and immediately before `handoff finish` triggers automatic projection",
			"bounded current-task inbox check",
			"numeric `sequence`",
			"exact `taskId` and `dispatchId`",
			"sender and recipient direction",
			"newly arrived current-task `status` or `escalation`",
			"through the observed maximum sequence",
			"repeat any affected verification and commit before finish",
			"fresh ready task, dispatch, host attestation, and sole-writer proof",
			"Conventional Commit subject",
			"literal `Lore:` block",
			"`Intent`, `Why`, `Changes`, `Verify`, and `Risk`",
			"Hooks may only observe, block, or relay",
			"must never execute the workflow",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s completion fence contract missing %q", name, want)
			}
		}
	}
}

func TestSupervisedHandoffSkillsRequireBoundedTaskAttestation(t *testing.T) {
	issueOps := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	turing := readTuringSkillForTest(t)
	for name, body := range map[string]string{"IssueOps": issueOps, "Turing": turing} {
		for _, want := range []string{
			"server-filtered task inventory",
			"orca orchestration task-list --status dispatched --json",
			"orca orchestration dispatch-show --task <current-task-id> --json",
			"`orca orchestration task show`, `orca orchestration dispatch show`, and status `in_progress` are invalid",
			"truncated or unparsable JSON is ambiguity, never absence",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s bounded task attestation contract missing %q", name, want)
			}
		}
	}
}

func TestSupervisedHandoffSkillsSeparateMailboxMessagingFromTerminalControl(t *testing.T) {
	issueOps := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	turing := readTuringSkillForTest(t)
	for name, body := range map[string]string{"IssueOps": issueOps, "Turing": turing} {
		for _, want := range []string{
			"Orchestration reviews and task-scoped messages target the sealed `WorkerMailboxHandle`",
			"refreshed `WorkerTerminalHandle` is only terminal read/send/close control",
			"Automatic `worker_done` uses the sealed `WorkerMailboxHandle` as `--from`",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s mailbox/control distinction missing %q", name, want)
			}
		}
	}
}

func TestIssueOpsSkillDocumentsExpectedContextSHA256PreviewConfirmFlow(t *testing.T) {
	issueOps := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	for _, want := range []string{
		"Preview returns `context_sha256`",
		"--expected-context-sha256",
		"all delivery options remain identical",
		"missing, malformed, or differs",
		"before any terminal, task, dispatch, or journal mutation",
	} {
		if !strings.Contains(issueOps, want) {
			t.Fatalf("IssueOps supervised handoff recipe missing %q", want)
		}
	}
}

func TestTuringSkillRequiresCodeGraphForIndexedLocalReposAndSeparateCalls(t *testing.T) {
	turing := readTuringSkillForTest(t)
	for _, want := range []string{
		"CodeGraph first when `.codegraph/` exists",
		"local `rg` and direct reads only",
		"Never use web search for local repository symbols",
		"separate calls",
		"never chain them with `echo` or `printf` banner markers",
	} {
		if !strings.Contains(turing, want) {
			t.Fatalf("Turing local-symbol discovery contract missing %q", want)
		}
	}
}

func TestIssueOpsSkillPinsGitLabSupervisedHandoffContract(t *testing.T) {
	issueOps := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	for _, want := range []string{
		"GitHub-only `--issue` flag",
		"GitLab omits `--issue`",
		"never invents a GitLab flag",
		"linkedGitLabIssue",
		"orca_gitlab_native_metadata_unavailable",
		"The observation is durable",
		"no Orca warning or handoff state",
		"sealed context must show the exact verified provider and Issue URL",
	} {
		if !strings.Contains(issueOps, want) {
			t.Fatalf("IssueOps GitLab supervised handoff contract missing %q", want)
		}
	}

	turing := readTuringSkillForTest(t)
	for _, duplicated := range []string{
		"GitHub-only `--issue` flag",
		"GitLab omits `--issue`",
		"linkedGitLabIssue",
		"orca_gitlab_native_metadata_unavailable",
		"no Orca warning or handoff state",
	} {
		if strings.Contains(turing, duplicated) {
			t.Fatalf("Turing duplicates canonical IssueOps GitLab handoff rule %q", duplicated)
		}
	}
}

func TestIssueOpsSkillUsesNumericMailboxSequenceInsteadOfOpaqueMessageIDs(t *testing.T) {
	issueOps := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	for _, want := range []string{
		"Never order or filter opaque `msg_*` IDs",
		"numeric `sequence`",
		"exact `taskId`, `dispatchId`, sender and recipient direction",
		"Sequence is evidence, not a lease fence",
		"`.result.messages`",
	} {
		if !strings.Contains(issueOps, want) {
			t.Fatalf("IssueOps numeric mailbox-selection contract missing %q", want)
		}
	}
}

func TestSupervisedHandoffSkillsNeverPredictMailboxSequence(t *testing.T) {
	for name, body := range map[string]string{
		"IssueOps": readIssueOpsReferenceForTest(t, "orca-handoff.md"),
		"Turing":   readTuringSkillForTest(t),
	} {
		for _, want := range []string{
			"Never prestate or predict a future mailbox sequence",
			"only the returned send envelope or a subsequent bounded mailbox observation supplies the sequence",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s mailbox-sequence provenance contract missing %q", name, want)
			}
		}
	}
}

func TestRootAgentGuidanceRunsRealGoldenPackage(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	guidance := string(body)
	if strings.Contains(guidance, "go test ./cmd/harness -run Golden") {
		t.Fatal("root AGENTS must not prescribe a package with zero matching golden tests")
	}
	if !strings.Contains(guidance, "go test ./cmd/harness/contractgolden -run Golden -count=1") {
		t.Fatal("root AGENTS must run the real contractgolden package")
	}
}

func TestSupervisedReportOnlySkillsRunOnlyDeclaredVerification(t *testing.T) {
	issueOps := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	turing := readTuringSkillForTest(t)
	for name, body := range map[string]string{"IssueOps": issueOps, "Turing": turing} {
		for _, want := range []string{
			"For a report-only cycle, run only the verification commands declared in the sealed worker packet",
			"Do not invent API, provider-ref, or history probes",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s report-only verification contract missing %q", name, want)
			}
		}
	}
}

func TestSupervisedAutomaticWorkerDoneSkillsPinExactAuthority(t *testing.T) {
	issueOps := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	turing := readTuringSkillForTest(t)
	for name, body := range map[string]string{"IssueOps": issueOps, "Turing": turing} {
		for _, want := range []string{
			"automatic best-effort projection",
			"submitted result and projection intent in the same cycle lock",
			"sealed historical worker mailbox",
			"refreshable live terminal",
			"exact persisted task and dispatch",
			"three-sentence body",
			"absolute in-worker report path",
			"never automatically retried",
			"manual shell `worker_done` is blocked",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s automatic worker_done contract missing %q", name, want)
			}
		}
		for _, forbidden := range []string{"Only the same submitted worker session may send", "retries the exact command once"} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("%s retains obsolete manual worker_done authority %q", name, forbidden)
			}
		}
	}
}

func TestIssueOpsSchemaADRPreservesV3HistoryAndAppendsCurrentDateV4(t *testing.T) {
	adr := readProjectContractFileForTest(t, ".agent-harness", "ADR.md")
	design := readProjectContractFileForTest(t, "docs", "superpowers", "specs", "2026-07-11-orca-aware-issueops-handoff-design.md")
	evidence := readProjectContractFileForTest(t, ".agent-harness", "research", "orca-handoff-turing-evidence-2026-07-11.md")
	legacyV3 := `## 2026-07-11 — IssueOps root schema v3 protects supervised ownership and stable terminal identity

- Kind: ` + "`adr`" + `
- Source: GitHub issue #16 schema compatibility review
- Summary: Stamp every IssueOps write as schema v3 so v1 cannot erase ` + "`execution_handoff`" + ` and v2 cannot erase the stable terminal tab/leaf locator needed after an Orca runtime rollover.
- Context: ` + "`execution_handoff`" + ` and stable terminal identity are not optional display metadata; they own mutation authority across host sessions. Leaving either addition under a schema already understood by an older writer lets that writer ignore the unknown field and weaken the guard during an unrelated read-modify-write.
- Decision:
  - Read missing, zero, v1, and v2 rows with the current model and preserve every recognized field; stamp v3 on the next write.
  - Reject versions greater than v3. For hook scans, retain only a bounded repo/worker identity projection and an in-memory invalid marker so unsupported rows remain fail-closed without being interpreted or rewritten.
  - Keep v3 visible at the root. Do not use a private migration table or infer compatibility from nested protocol_version.
- Consequences: CLI, MCP, daemon, and all native hosts must be updated together before mutating supervised rows. Compatibility fixtures must prove v1 rejects v2 and v2 rejects v3 byte-equivalently; install smoke must verify v1/v2 migration plus v3 readback.
- Evidence: ` + "`internal/core/issueops/issueops_schema_version_test.go`" + `, the real sqlstore future-schema lifecycle guard test, and three-host install migration verification recorded in the issue evidence ledger.
- Rejected alternatives: keeping schema v1 because the field is ` + "`omitempty`" + `; silently downgrading v2 for old binaries; discarding future-schema rows from hook ownership scans.`
	if !strings.Contains(adr, legacyV3) {
		t.Fatal("schema-v3 ADR history was rewritten instead of preserved byte-for-byte")
	}
	if !strings.Contains(adr, "## 2026-07-11 — IssueOps root schema v4 protects sealed completion authority") {
		t.Fatal("schema-v4 ADR was not appended as a separate current-date decision")
	}
	if strings.Contains(adr, "## 2026-07-12 — IssueOps root schema v4 protects sealed completion authority") {
		t.Fatal("schema-v4 ADR heading contains future-date drift")
	}
	if !strings.Contains(design, "**Status:** Implemented with the 2026-07-12 state/security correction") {
		t.Fatal("state/security design status has the wrong bundle date")
	}
	if !strings.Contains(evidence, "## Sealed automatic `worker_done` projection correction — 2026-07-11") || strings.Contains(evidence, "## Sealed automatic `worker_done` projection correction — 2026-07-12") {
		t.Fatal("sealed-projection evidence heading has the wrong bundle date")
	}
}

func TestCompletedHandoffPublicAPIRequiresProjectionDependency(t *testing.T) {
	for name, body := range map[string]string{
		"issueops lifecycle": readProjectContractFileForTest(t, "internal", "core", "issueops", "issueops_handoff_lifecycle.go"),
		"core facade":        readProjectContractFileForTest(t, "internal", "core", "issueops_facade.go"),
	} {
		if strings.Contains(body, "func FinishIssueOpsHandoff(") {
			t.Fatalf("%s still exports a completed-finish path without projection", name)
		}
	}
}

func TestIssueOpsCleanupUsesLiveTerminalAndExactWorktreeStop(t *testing.T) {
	body := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	for _, want := range []string{
		"orca terminal close --terminal <resolved-worker-terminal-handle> --json",
		"currently resolved `WorkerTerminalHandle`",
		"orca terminal stop --worktree id:<persisted-worktree-id> --json",
		"sealed `WorkerMailboxHandle` is never terminal-control authority",
		"remains the sealed orchestration `worker_done` sender",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("IssueOps cleanup contract missing %q", want)
		}
	}
	if strings.Contains(body, "terminal close --terminal <persisted-worker-mailbox-handle>") {
		t.Fatal("IssueOps cleanup still authorizes the historical mailbox as terminal control")
	}
}

func TestIssueOpsHistoricalSchemaDocsCarryFullV4RejectionChain(t *testing.T) {
	body := readProjectContractFileForTest(t, "docs", "superpowers", "specs", "2026-07-06-issueops-subagent-orchestration-design.md")
	want := "Current schema-v5 writers instead require v1 to reject v2+, v2 to reject v3, v3 to reject v4, and v4 to reject v5 before rewrite"
	if !strings.Contains(body, want) {
		t.Fatalf("historical schema design does not carry the full v4 rejection chain: want %q", want)
	}
	if strings.Contains(body, "Current schema-v3 writers") || strings.Contains(body, "Current schema-v4 writers") {
		t.Fatal("historical schema design describes an obsolete current writer")
	}
}

func TestIssueOpsSchemaV5DocsCarryPublicationCleanupAndFallbackAuthority(t *testing.T) {
	documents := map[string]string{
		"architecture": readProjectContractFileForTest(t, ".agent-harness", "ARCHITECTURE.md"),
		"operations":   readProjectContractFileForTest(t, ".agent-harness", "OPERATIONS.md"),
		"testing":      readProjectContractFileForTest(t, ".agent-harness", "TESTING.md"),
		"design":       readProjectContractFileForTest(t, "docs", "superpowers", "specs", "2026-07-11-orca-aware-issueops-handoff-design.md"),
		"plan":         readProjectContractFileForTest(t, "docs", "superpowers", "plans", "2026-07-11-orca-aware-issueops-handoff.md"),
	}
	for name, body := range documents {
		for _, want := range []string{"v5", "publish", "cleanup"} {
			if !strings.Contains(strings.ToLower(body), want) {
				t.Fatalf("%s schema-v5 authority contract missing %q", name, want)
			}
		}
	}
	reference := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	for _, want := range []string{
		"Every connected or writable terminal is a possible writer, including a pre-existing baseline terminal",
		"agent-harness issueops handoff publish",
		"agent-harness issueops remote create-pr",
		"approve-cleanup",
		"task_terminal",
		"terminal_quiescent",
		"worktree_removed",
		"byte-identical to the legacy inline path",
		"schema-v4 record with no `execution_handoff`",
		"Automatic publication-fence guarantees apply only to future supervised envelopes",
		"never authorizes raw worktree removal",
	} {
		if !strings.Contains(reference, want) {
			t.Fatalf("IssueOps v5 handoff contract missing %q", want)
		}
	}
	if strings.Contains(reference, "orca worktree rm --worktree id:<persisted-worktree-id> --force --json") {
		t.Fatal("IssueOps reference still claims automatic supervised worktree removal")
	}
	publish := reference[strings.Index(reference, "## Coordinator Publish"):strings.Index(reference, "## Failure And Recovery")]
	recipeStart := strings.Index(publish, "```bash\n")
	recipe := publish[recipeStart+len("```bash\n"):]
	recipe = recipe[:strings.Index(recipe, "```")]
	for _, forbidden := range []string{"git push", "gh pr create", "glab mr create", "--body-file"} {
		if strings.Contains(recipe, forbidden) {
			t.Fatalf("publish recipe retains forbidden surface %q", forbidden)
		}
	}
}

func TestSupervisedHandoffTreatsUsageAndModelPromptsAsUserDecisionBoundary(t *testing.T) {
	for name, body := range map[string]string{
		"IssueOps skill":     readIssueOpsSkillForTest(t),
		"IssueOps reference": readIssueOpsReferenceForTest(t, "orca-handoff.md"),
	} {
		for _, want := range []string{"usage-limit", "rate-limit", "reset", "model-selection", "user-decision", "dismiss or stop", "never", "switch models"} {
			if !strings.Contains(strings.ToLower(body), want) {
				t.Fatalf("%s usage/model boundary missing %q", name, want)
			}
		}
	}
}

func TestIssueOpsSchemaV4DocsCarryAttemptWideMailboxMigrationAndPairing(t *testing.T) {
	documents := map[string]string{
		"operations": readProjectContractFileForTest(t, ".agent-harness", "OPERATIONS.md"),
		"cautions":   readProjectContractFileForTest(t, ".agent-harness", "CAUTIONS.md"),
		"testing":    readProjectContractFileForTest(t, ".agent-harness", "TESTING.md"),
		"design":     readProjectContractFileForTest(t, "docs", "superpowers", "specs", "2026-07-11-orca-aware-issueops-handoff-design.md"),
		"plan":       readProjectContractFileForTest(t, "docs", "superpowers", "plans", "2026-07-11-orca-aware-issueops-handoff.md"),
		"evidence":   readProjectContractFileForTest(t, ".agent-harness", "research", "orca-handoff-turing-evidence-2026-07-11.md"),
	}
	for name, body := range documents {
		for _, want := range []string{
			"current attempt and every prior attempt",
			"`DispatchID` and `WorkerMailboxHandle` are either both absent or both present",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s schema-v4 authority contract missing %q", name, want)
			}
		}
	}
}

func TestTuringVerificationWaitsForYieldedCellsAndUsesExplicitGofmtArgv(t *testing.T) {
	body := readTuringSkillForTest(t)
	for _, want := range []string{
		"A yielded execution cell is unfinished evidence",
		"through a terminal exit",
		"restart the ordered verification gate from step 1",
		"Never construct `gofmt -w` arguments with shell command substitution",
		"explicit direct argv list",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("Turing verification process contract missing %q", want)
		}
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
	for _, want := range []string{"git rev-parse --verify refs/heads/<branch>", "agent-harness issueops handoff publish", "agent-harness issueops remote create-pr"} {
		if !strings.Contains(recipe, want) {
			t.Fatalf("publish recipe missing %q", want)
		}
	}
	for _, forbidden := range []string{"git push", "gh pr create", "glab mr create", "--body-file"} {
		if strings.Contains(recipe, forbidden) {
			t.Fatalf("publish recipe retains forbidden surface %q", forbidden)
		}
	}
}

func TestIssueOpsCancellationRecoveryKeepsLeaseUntilExactQuiescence(t *testing.T) {
	body := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	for _, want := range []string{
		"--action finalize-cancel --confirm",
		"cancellation tombstone",
		"exact terminal disconnected or absent",
		"exact task/dispatch terminal or authoritatively absent",
		"unique stable identity",
		"missing, duplicate, or incomplete rows remain ambiguous",
		"--action abandon --confirm --force --reason",
		"successfully force-abandoned attempt is not retryable",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("IssueOps cancellation recovery contract missing %q", want)
		}
	}
}

func TestIssueOpsSupervisedStartDocumentsReviewedContextCASAndProgressReporting(t *testing.T) {
	body := readIssueOpsReferenceForTest(t, "orca-handoff.md")
	for _, want := range []string{
		"--expected-context-sha256",
		"final attested preview",
		"fails closed before any terminal, task, dispatch, or journal mutation",
		"--type status",
		"--task-id",
		"--dispatch-id",
		"`task_bounded` is not a message type",
		"subject label",
		"selection evidence, not part of the lease fence",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("IssueOps supervised start CAS/progress contract missing %q", want)
		}
	}
}

func TestIssueOpsSupervisedStartDocumentsFinalizeConfirmedCASWording(t *testing.T) {
	for name, body := range map[string]string{
		"turing":   readTuringSkillForTest(t),
		"cautions": readCautionsForTest(t),
	} {
		for _, want := range []string{
			"Final confirmed start must add `--expected-context-sha256` with the exact final attested preview hash plus `--confirm`; all delivery options remain identical.",
			"exact final attested preview hash",
			"all delivery options remain identical",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s supervised handoff CAS wording missing %q", name, want)
			}
		}
	}
}

func TestSupervisedHandoffSkillsPinOrcaEnvironmentKeyAllowlist(t *testing.T) {
	for name, body := range map[string]string{
		"IssueOps": readIssueOpsReferenceForTest(t, "orca-handoff.md"),
		"Turing":   readTuringSkillForTest(t),
		"CAUTIONS": readCautionsForTest(t),
	} {
		for _, want := range []string{
			"Explicit nonsecret Orca environment-key allowlist",
			"broad ORCA-prefixed env output",
			"prefix filtering",
			"ORCA_TERMINAL_HANDLE",
			"ORCA_TAB_ID",
			"ORCA_WORKTREE_ID",
			"never record secret values",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s environment allowlist contract missing %q", name, want)
			}
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

func readCautionsForTest(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", ".agent-harness", "CAUTIONS.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func readProjectContractFileForTest(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{"..", "..", ".."}, parts...)...)
	b, err := os.ReadFile(path)
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
