package issueopsnext

import "strings"

// OwnerCommand는 readiness가 돌려준 missing 키 하나를 그 상태를 소유한 명령으로
// 옮긴다. 이 함수가 그 대응의 유일한 소유자다 — 같은 표를 스킬 문서에도 두면
// 한쪽만 낡는다.
//
// 반환 문자열의 `<...>`는 사람이 채울 자리표시자이며, 그 존재가 곧
// next_command_kind=template이다. 알 수 없는 키는 추측하지 않고 status로 보낸다.
func OwnerCommand(id, missingKey string) string {
	key := strings.TrimSpace(missingKey)
	switch {
	case strings.HasPrefix(key, "plan_prep_"):
		return "agent-harness issueops plan-prep record --id " + id +
			" --decisions-evidence <TEXT> --related-score-ref <TEXT> --web-research-evidence <TEXT> --codebase-survey-evidence <TEXT>"
	case strings.HasPrefix(key, "design_"):
		return designReviewCommand(id)
	case strings.HasPrefix(key, "compatibility_"):
		return compatibilityReviewCommand(id)
	case strings.HasPrefix(key, "ai_slop_clean_"):
		return aiSlopCleanCommand(id)
	case strings.HasPrefix(key, "gates_incomplete:"):
		return "agent-harness gates check --cwd <worktree> --workspace-root <worktree> --json"
	case strings.HasPrefix(key, "duplicate_issue_artifact:"):
		return "agent-harness issueops remote reconcile-issue --id " + id
	}
	switch key {
	case "intent_contract", "raw_request", "interpreted_intent", "success_criteria":
		return "agent-harness issueops intent record --id " + id +
			" --raw-request <TEXT> --interpreted-intent <TEXT> --success-criteria <TEXT>"
	case "domain_review":
		return "agent-harness issueops domain-review record --id " + id + " --model-fit <TEXT>"
	case "split_decision":
		return "agent-harness issueops decision add --id " + id +
			" --kind scope --title <TEXT> --body <TEXT>"
	case "issue_url":
		return "agent-harness issueops link-issue --id " + id + " --issue-url <URL>"
	case "branch", "branch_prepare":
		return "agent-harness issueops branch prepare --id " + id +
			" --provider <github|gitlab> --issue-url <URL> --branch <NAME> --base-branch <REF> --base-sha <SHA> --link-verified"
	case "design_review":
		return designReviewCommand(id)
	case "compatibility_review":
		return compatibilityReviewCommand(id)
	case "devils_advocate_review", "devils_advocate_review_stale":
		return "agent-harness issueops devils-advocate review --id " + id +
			" --reviewer-context subagent --verdict <pass|revise|stop> --finding <TEXT>"
	case "plan_artifact":
		return "agent-harness issueops artifact stage --id " + id + " --name plan --file <PATH>"
	case "plan_path", "plan_exists", "plan_in_worktree":
		return "agent-harness issueops link-plan --id " + id + " --plan-path <PATH>"
	case "worktree_path", "worktree_exists":
		return "agent-harness issueops execution prepare --id " + id + " --mode auto --owner-host <codex|claude|omo> $ACTOR_FLAGS"
	case "execution", "execution_valid", "execution_worktree_match", "execution_write_lease":
		return executionStatusCommand(id)
	case "implementation_changes":
		return statusCommand(id)
	case "cleanup_evidence", "verification_evidence", "current_fingerprint":
		return aiSlopCleanCommand(id)
	case "implementation_review", "implementation_review_stale":
		return "agent-harness issueops implementation-review record --id " + id +
			" --verdict <pass|revise|stop> --finding <TEXT> --evidence <TEXT>"
	case "project_docs_review", "project_docs_review_stale":
		return "agent-harness issueops project-docs-review record --id " + id +
			" --verdict <updated|no-change> --evidence <TEXT>"
	case "schema_evidence", "schema_evidence_stale":
		return "agent-harness issueops schema-evidence record --id " + id +
			" --measurement <TEXT> --source <TEXT>"
	case "feedback_classification":
		return "agent-harness issueops feedback add --id " + id +
			" --source <TEXT> --body <TEXT> --classification <TEXT>"
	case "feedback_resolution":
		return "agent-harness issueops feedback resolve --id " + id +
			" --index <N> --resolution <valid-defect|question-answered|noise-dismissed>"
	case "contract_feedback_issue_update":
		return "agent-harness issueops feedback mark-issue-updated --id " + id
	case "remote_artifact":
		return "agent-harness issueops remote verify-artifact --id " + id +
			" --provider <github|gitlab> --kind <pr|mr> --url <URL> --target-branch <BRANCH> --label <LABEL> --assignee <USER>"
	case "child_incomplete", "child_unvalidated", "child_rejected_unresolved":
		return "agent-harness issueops child status --parent " + id + " --json"
	case "worktree_clean", "upstream", "upstream_fetch", "upstream_synced", "branch_match":
		return "agent-harness issueops pr-readiness --id " + id + " --strict --json"
	default:
		return statusCommand(id)
	}
}

func designReviewCommand(id string) string {
	return "agent-harness issueops design review --id " + id +
		" --problem-summary <TEXT> --proposed-design <TEXT> --refactor-plan <TEXT>" +
		" --alternative <TEXT> --risk <TEXT> --verification <TEXT> --approved"
}

func compatibilityReviewCommand(id string) string {
	return "agent-harness issueops compatibility review --id " + id +
		" --backward-compatibility <TEXT> --side-effect <TEXT> --rollback-plan <TEXT> --verification <TEXT> --approved"
}

func aiSlopCleanCommand(id string) string {
	return "agent-harness issueops ai-slop-clean record --id " + id +
		" --category <TEXT> --verification <TEXT>"
}

func executionStatusCommand(id string) string {
	return "agent-harness issueops execution status --id " + id + " --json"
}

func statusCommand(id string) string {
	return "agent-harness issueops status --id " + id + " --json"
}
