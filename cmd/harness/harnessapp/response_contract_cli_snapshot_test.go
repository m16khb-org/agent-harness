package harnessapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/cmd/harness/harnessapp/responsecontract"
	"agent-harness/internal/core"
	"agent-harness/internal/core/sqlstore"
)

func buildCLIResponseContractSnapshot(t *testing.T, replacements map[string]string, stateDir, workspaceDir, gitRepoDir string) map[string]any {
	t.Helper()
	installFakeGlabForDevilsAdvocateReflect(t)
	cliSnapshot := map[string]any{}
	cliSnapshot["inspect"] = inspectContractProjection(runCLIJSONContract(t, replacements, func() error {
		return runInspect([]string{"--json", "--repo", workspaceDir})
	}))
	cliSnapshot["docs_index"] = docsIndexContractProjection(runCLIJSONContract(t, replacements, func() error {
		return runDocs([]string{"--json"})
	}))
	cliSnapshot["daemon_status"] = runCLIJSONContract(t, replacements, func() error {
		return runDaemon([]string{"status", "--json"})
	})
	cliSnapshot["preflight"] = runCLIJSONContract(t, replacements, func() error {
		return runPreflight([]string{"--json", gitRepoDir})
	})
	cliSnapshot["verify_work"] = runCLIJSONContract(t, replacements, func() error {
		return runVerifyWork([]string{"--repo", gitRepoDir, "--json", "--", "git", "status", "--short"})
	})
	cliSnapshot["quality_inspect"] = runCLIJSONContract(t, replacements, func() error {
		return runQualityInspectWithDeps([]string{"--repo", workspaceDir, "--json"}, qualitycliInspectDepsForContract())
	})
	cliSnapshot["policy_check"] = runCLIJSONContract(t, replacements, func() error {
		return runPolicy([]string{"check", "--workspace-root", workspaceDir, "--cwd", workspaceDir, "--json", "--", "git", "status", "--short"})
	})
	cliSnapshot["state_write"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"write", "--key", "current", "--value", "current content", "--json"})
	})
	cliSnapshot["state_read"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"read", "--key", "current", "--json"})
	})
	cliSnapshot["state_list"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"list", "--json"})
	})
	issueopsStartStdout := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"start", "--repo", workspaceDir, "--branch", "1-contract-branch", "--json"})
	})
	var issueopsStartRaw map[string]any
	if err := json.Unmarshal([]byte(issueopsStartStdout), &issueopsStartRaw); err != nil {
		t.Fatalf("unmarshal issueops start JSON %q: %v", issueopsStartStdout, err)
	}
	cliSnapshot["issueops_start"] = responsecontract.NormalizeContractValue(issueopsStartRaw, replacements)
	issueopsID, ok := issueopsStartRaw["id"].(string)
	if !ok || issueopsID == "" {
		t.Fatalf("issueops start missing id: %#v", issueopsStartRaw)
	}
	replacements[issueopsID] = "$ISSUEOPS_ID"
	cliSnapshot["issueops_record_intent"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"intent", "record", "--id", issueopsID, "--raw-request", "Refactor IssueOps intent flow", "--interpreted-intent", "Persist main-agent judgment before planning", "--success-criteria", "intent is recorded", "--constraint", "keep contract deterministic", "--ambiguity", "none", "--non-goal", "do not continue from hook recommendation alone", "--json"})
	})
	cliSnapshot["issueops_plan_prep_record"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"plan-prep", "record", "--id", issueopsID, "--decisions-evidence", ".agent-harness/ADR.md", "--related-score-ref", "remote score: selected=#1(0.81), threshold=0.70", "--web-research-evidence", ".agent-harness/research/contract.md", "--codebase-survey-evidence", "rg/CodeGraph sweep: issueops readiness, plan-prep recorder, CLI flags", "--json"})
	})
	cliSnapshot["issueops_link_issue"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"link-issue", "--id", issueopsID, "--issue-url", "https://gitlab.example/group/project/-/issues/1", "--json"})
	})
	cliSnapshot["issueops_prepare_branch"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"branch", "prepare", "--id", issueopsID, "--provider", "gitlab", "--issue-url", "https://gitlab.example/group/project/-/issues/1", "--branch", "1-contract-branch", "--base-branch", "main", "--link-verified", "--json"})
	})
	contractWorktree := filepath.Join(filepath.Dir(workspaceDir), filepath.Base(workspaceDir)+".worktrees", "1-contract-branch")
	if err := os.MkdirAll(filepath.Join(contractWorktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(contractWorktree, ".git", "HEAD"), []byte("ref: refs/heads/1-contract-branch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	replacements[contractWorktree] = "$ISSUEOPS_WORKTREE"
	addEvalSymlinkReplacement(t, replacements, contractWorktree, "$ISSUEOPS_WORKTREE")
	cliSnapshot["issueops_link_worktree"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"link-worktree", "--id", issueopsID, "--worktree-path", contractWorktree, "--json"})
	})
	cliSnapshot["issueops_review_design"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"design", "review", "--id", issueopsID, "--problem-summary", "IssueOps needs explicit design review", "--proposed-design", "Gate implementation on approved design", "--refactor-plan", "Keep changes local to IssueOps state and adapters", "--risk", "golden contract drift", "--alternative", "docs-only guidance", "--verification", "design review checked contract drift risk", "--verification", "go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden", "--approved", "--json"})
	})
	writeContractFile(t, contractWorktree, "docs/superpowers/plans/contract.md", "plan\n")
	cliSnapshot["issueops_link_plan"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"link-plan", "--id", issueopsID, "--plan-path", filepath.Join(contractWorktree, "docs", "superpowers", "plans", "contract.md"), "--json"})
	})
	cliSnapshot["issueops_link_child"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"link-child", "--id", issueopsID, "--child-url", "https://gitlab.example/group/project/-/issues/2", "--title", "contract child", "--json"})
	})
	cliSnapshot["issueops_link_related"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"link-related", "--id", issueopsID, "--type", "depends-on", "--related-url", "https://github.com/example/repo/issues/42", "--title", "upstream dependency", "--json"})
	})
	cliSnapshot["issueops_decision_add"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"decision", "add", "--id", issueopsID, "--title", "Contract decision", "--body", "Chose approach A over B for contract snapshot coverage", "--kind", "architecture", "--rationale", "Approach A keeps snapshots deterministic", "--alternative", "Approach B: live-only verification", "--alternative", "Approach C: manual review", "--affected-artifact", "test", "--affected-artifact", "implementation", "--json"})
	})
	executionContractID := seedIssueOpsExecutionContract(t, workspaceDir, "69-cli-execution-contract")
	replacements[executionContractID] = "$CLI_EXECUTION_ID"
	cliSnapshot["issueops_execution_status"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"execution", "status", "--id", executionContractID, "--json"})
	})
	cliSnapshot["issueops_feedback_add"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"feedback", "add", "--id", issueopsID, "--source", "user", "--body", "tighten contract", "--classification", "contract_change", "--json"})
	})
	cliSnapshot["issueops_mark_issue_updated"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"feedback", "mark-issue-updated", "--id", issueopsID, "--json"})
	})
	cliSnapshot["issueops_pr_readiness"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"pr-readiness", "--id", issueopsID, "--json"})
	})
	// Phase-ledger tools. The cycle is already at plan (link-issue auto-advances
	// once plan readiness is met), so the sequence exercises real transitions
	// before force-release ends the cycle in done: status (derived ledger at plan)
	// -> regress (Brooks stop, plan -> grill, stale-marks the ledger) ->
	// domain-review record (re-satisfies the grill gate) -> set-phase (grill ->
	// plan, a real forward transition that stamps the ledger) -> ai-slop-clean
	// evidence -> feedback resolve.
	cliSnapshot["issueops_status"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"status", "--id", issueopsID, "--json"})
	})
	cliSnapshot["issueops_record_devils_advocate_review"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"devils-advocate", "review", "--id", issueopsID, "--verdict", "stop", "--finding", "second-system effect: three cache backends for one need", "--json"})
	})
	cliSnapshot["issueops_remote_reflect_devils_advocate"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"remote", "reflect-devils-advocate", "--id", issueopsID, "--confirm", "--json"})
	})
	cliSnapshot["issueops_regress_for_replan"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"regress", "--id", issueopsID, "--reason", "brooks stop: scope too broad for one cycle", "--json"})
	})
	cliSnapshot["issueops_domain_review"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"domain-review", "record", "--id", issueopsID, "--model-fit", "fits the IssueOps phase-ledger domain model", "--terminology", "phase ledger", "--risk", "ledger drift between CLI and MCP", "--json"})
	})
	cliSnapshot["issueops_set_phase"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"phase", "--id", issueopsID, "--to", "plan", "--json"})
	})
	cliSnapshot["issueops_ai_slop_clean"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"ai-slop-clean", "record", "--id", issueopsID, "--category", "naming", "--verification", "go test ./cmd/harness/harnessapp -run Golden", "--json"})
	})
	cliSnapshot["issueops_feedback_resolve"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"feedback", "resolve", "--id", issueopsID, "--index", "0", "--resolution", "valid-defect", "--json"})
	})
	cliSnapshot["issueops_remote_create_issue"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"remote", "create-issue", "--id", issueopsID, "--title", "contract issue", "--body", "contract body", "--label", "contract", "--json"})
	})
	cliSnapshot["issueops_remote_create_child"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"remote", "create-child", "--id", issueopsID, "--title", "contract child", "--body", "contract child body", "--label", "contract", "--assignee", "octocat", "--json"})
	})
	cliSnapshot["issueops_remote_create_pr"] = runCLIJSONContract(t, replacements, func() error {
		return runIssueOps([]string{"remote", "create-pr", "--id", executionContractID, "--expected-generation", "1", "--title", "contract PR", "--head", "69-cli-execution-contract", "--base", "main", "--label", "contract", "--assignee", "octocat", "--json"})
	})

	old := mustStateReadForContract(t, "current")
	old.Record.Key = "old"
	old.Record.UpdatedAt = "2000-01-01T00:00:00Z"
	mustWriteStateRecordForContract(t, stateDir, "old", old.Record)
	cliSnapshot["state_prune_dry_run"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"prune", "--max-age", "1h", "--json"})
	})
	cliSnapshot["state_prune_confirm"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"prune", "--max-age", "1h", "--confirm", "--json"})
	})

	legacy := core.StateRecord{
		Key:       "legacy",
		Content:   "legacy content",
		UpdatedAt: "2000-01-01T00:00:00Z",
		Bytes:     len([]byte("legacy content")),
	}
	mustWriteStateRecordForContract(t, stateDir, "legacy", legacy)
	cliSnapshot["state_migrate_dry_run"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"migrate", "--json"})
	})
	cliSnapshot["state_migrate_confirm"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"migrate", "--confirm", "--json"})
	})
	cliSnapshot["state_doctor_healthy"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"doctor", "--json"})
	})

	if db, err := sqlstore.Open(stateDir); err != nil {
		t.Fatal(err)
	} else if err := db.Put("state", "corrupt", []byte("{not json\n")); err != nil {
		t.Fatal(err)
	}
	cliSnapshot["state_doctor_corrupt"] = runCLIJSONContract(t, replacements, func() error {
		return runState([]string{"doctor", "--json"})
	})

	writeSelfAugmentCompareFixturesForContract(t, stateDir)
	cliSnapshot["self_augment_plan"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfAugment([]string{"--target-score", "95", "--json"})
	})
	cliSnapshot["self_augment_lesson"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfAugment([]string{"lesson", "--candidate", "reflexion-state-memory", "--lesson", "Contract lesson", "--next-action", "Check stored lesson before next cycle", "--state-key", "self-augment-lesson-contract", "--json"})
	})
	cliSnapshot["self_verify_candidates"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfVerify([]string{"candidates", "--save-state", "--state-key", "self-verify-candidates-contract", "--json"})
	})
	cliSnapshot["self_verify_compare"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfVerify([]string{"compare", "--baseline-key", "self-verify-baseline", "--candidate-key", "self-verify-candidate", "--json"})
	})
	cliSnapshot["self_verify_promote_dry_run"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfVerify([]string{"promote", "--from-key", "self-verify-candidate", "--baseline-key", "self-verify-promoted", "--json"})
	})
	cliSnapshot["self_verify_history"] = runCLIJSONContract(t, replacements, func() error {
		return runSelfVerify([]string{"history", "--prefix", "self-verify", "--json"})
	})
	cliSnapshot["contract_schema"] = runCLIJSONContract(t, replacements, func() error {
		return runContract([]string{"schema", "--json"})
	})
	cliSnapshot["contract_conformance_baseline"] = runCLIJSONContract(t, replacements, func() error {
		return runContract([]string{"conformance", "baseline", "--json"})
	})
	traceInput := filepath.Join(workspaceDir, "trace.jsonl")
	if err := os.WriteFile(traceInput, []byte(`{"kind":"code_change","target_docs":["OPERATIONS.md"],"summary":"contract fixture","source":"contract"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cliSnapshot["trace_analyze"] = runCLIJSONContract(t, replacements, func() error {
		return runTrace([]string{"analyze", "--input", traceInput, "--json"})
	})
	cliSnapshot["policy_audit"] = runCLIJSONContract(t, replacements, func() error {
		return runPolicy([]string{"audit", "--workspace-root", workspaceDir, "--cwd", workspaceDir, "--json", "--", "git", "status", "--short"})
	})
	cliSnapshot["worker_enqueue"] = runCLIJSONContract(t, replacements, func() error {
		return runWorker([]string{"enqueue", "--kind", "contract", "--payload", "TOKEN=secret-value", "--json"})
	})
	return cliSnapshot
}

// installFakeGlabForDevilsAdvocateReflect puts a fake glab on PATH so the
// reflect-devils-advocate step can round-trip the gitlab issue description
// without a real network call. It answers the REST GET with a body and the PUT
// with success.
func installFakeGlabForDevilsAdvocateReflect(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	script := `#!/bin/sh
case "$*" in
  *"--method PUT"*)
    printf '{"web_url":"https://gitlab.example/group/project/-/issues/1"}'
    exit 0
    ;;
esac
printf '{"description":"contract issue body","web_url":"https://gitlab.example/group/project/-/issues/1"}'
exit 0
`
	if err := os.WriteFile(filepath.Join(bin, "glab"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}
