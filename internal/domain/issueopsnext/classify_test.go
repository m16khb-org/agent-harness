package issueopsnext

import (
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
)

func defaultCompletion(phase issueopsnextcontract.Phase) Readiness {
	switch phase {
	case issueopsnextcontract.PhaseProblem:
		return Readiness{Missing: []string{"intent_contract"}}
	case issueopsnextcontract.PhaseGrill:
		return Readiness{Missing: []string{"branch", "issue_url"}}
	case issueopsnextcontract.PhaseImplement:
		return Readiness{Missing: []string{"devils_advocate_review"}}
	default:
		return Readiness{Ready: true}
	}
}

func baseRecord(phase issueopsnextcontract.Phase) Input {
	return Input{
		Record: issueopsnextcontract.Record{
			ID: "io-test", Repo: "/repo", Branch: "12-test", Phase: phase,
		},
		Completion: defaultCompletion,
		SourceRoot: "/repo",
		ActorHost:  "claude", ActorSessionID: "s-1",
	}
}

func withIssue(in Input) Input {
	in.Record.IssueURL = "https://github.com/acme/repo/issues/12"
	return withoutCompletionKey(in, issueopsnextcontract.PhaseGrill, "issue_url")
}

func withBranch(in Input) Input {
	in.Record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: in.Record.IssueURL, Branch: in.Record.Branch,
		BaseBranch: "main", BaseSHA: "a1b2c3d4", LinkVerified: true,
	}
	return withoutCompletionKey(in, issueopsnextcontract.PhaseGrill, "branch")
}

func withStagedPlan(in Input) Input {
	in.StagedPlan = true
	return in
}

func withDesign(in Input) Input {
	in.Record.DesignReview = &issueopscontract.IssueOpsDesignReview{
		ProblemSummary: "summary", ProposedDesign: "design",
		Verification: []string{"design review checked alternatives and risks"},
		Approved:     true, ReviewedAt: "2026-09-05T00:00:00Z",
	}
	return in
}

func withDA(in Input) Input {
	in.Record.DevilsAdvocateReview = &issueopscontract.IssueOpsDevilsAdvocateReview{
		Verdict: "pass", ReviewedPlanDigest: "digest", RecordedAt: "2026-09-05T00:01:00Z",
	}
	return withoutCompletionKey(in, issueopsnextcontract.PhaseImplement, "devils_advocate_review")
}

func withDAStop(in Input) Input {
	in.Record.DevilsAdvocateReview = &issueopscontract.IssueOpsDevilsAdvocateReview{
		Verdict: "stop", Findings: []string{"finding"}, Waived: false,
		ReviewedPlanDigest: "digest", RecordedAt: "2026-09-05T00:01:00Z",
	}
	return in
}

func withExecution(in Input, lease issueopscontract.WriteLease) Input {
	in.Record.Execution = &issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeDirect,
		Workspace: issueopscontract.Workspace{
			SourceRoot: "/repo", Root: "/repo.worktrees/12-test", Branch: in.Record.Branch,
		},
		Lease: lease,
	}
	in.WorktreePresent = true
	return in
}

func withSelfHolder(in Input) Input {
	return withExecution(in, issueopscontract.WriteLease{
		Generation: 1, Status: issueopsnextcontract.LeaseStatusActive,
		Holder: &issueopscontract.NativeActor{Host: "claude", SessionID: "s-1"},
	})
}

func withHolder(in Input, host, session string, alive bool) Input {
	in = withExecution(in, issueopscontract.WriteLease{
		Generation: 1, Status: issueopsnextcontract.LeaseStatusActive,
		Holder: &issueopscontract.NativeActor{Host: host, SessionID: session},
	})
	in.HolderLive = &alive
	return in
}

func withReleasedLease(in Input) Input {
	return withExecution(in, issueopscontract.WriteLease{
		Generation: 1, Status: issueopsnextcontract.LeaseStatusReleased,
	})
}

func withClaimableLease(in Input) Input {
	return withExecution(in, issueopscontract.WriteLease{
		Generation: 1, Status: issueopsnextcontract.LeaseStatusClaimable,
	})
}

func withPending(in Input) Input {
	in = withSelfHolder(in)
	in.Record.Execution.Pending = &issueopscontract.ExternalIntent{
		OperationID: "op-1", Kind: "create_issue", Marker: "marker", StartedAt: "2026-09-05T00:02:00Z",
	}
	return in
}

func withArtifact(in Input) Input {
	in.Record.RemoteArtifact = &issueopscontract.IssueOpsRemoteArtifactVerification{
		Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/12",
		VerifiedAt: "2026-09-05T00:03:00Z",
	}
	return in
}

func withCompletionMissing(in Input, phase issueopsnextcontract.Phase, keys ...string) Input {
	inner := in.Completion
	in.Completion = func(candidate issueopsnextcontract.Phase) Readiness {
		if candidate == phase {
			return Readiness{Ready: len(keys) == 0, Missing: keys}
		}
		return inner(candidate)
	}
	return in
}

func withoutCompletionKey(in Input, phase issueopsnextcontract.Phase, key string) Input {
	inner := in.Completion
	in.Completion = func(candidate issueopsnextcontract.Phase) Readiness {
		readiness := inner(candidate)
		if candidate != phase {
			return readiness
		}
		var kept []string
		for _, item := range readiness.Missing {
			if item != key {
				kept = append(kept, item)
			}
		}
		readiness.Missing = kept
		readiness.Ready = len(kept) == 0
		return readiness
	}
	return in
}

func withLocal(in Input, keys ...string) Input {
	in.Local = &Readiness{Ready: len(keys) == 0, Missing: keys}
	return in
}

func TestClassifyStages(t *testing.T) {
	cases := []struct {
		name string
		in   Input
		key  string
		idx  int
	}{
		{"no record", Input{}, "none", 0},
		{"pending intent blocks", withPending(baseRecord(issueopsnextcontract.PhaseImplement)), "blocked.pending", 4},
		{"other live holder", withHolder(baseRecord(issueopsnextcontract.PhaseImplement), "codex", "s-2", true), "blocked.holder_live", 4},
		{"stale holder takeover", withHolder(baseRecord(issueopsnextcontract.PhaseImplement), "codex", "s-2", false), "takeover", 4},
		{"grill without issue", baseRecord(issueopsnextcontract.PhaseGrill), "issue", 1},
		{"grill with issue no branch", withIssue(baseRecord(issueopsnextcontract.PhaseGrill)), "prepare", 2},
		{"plan right after link-issue auto-advance", withIssue(baseRecord(issueopsnextcontract.PhasePlan)), "prepare", 2},
		{"implement without execution after switch-mode", withIssue(baseRecord(issueopsnextcontract.PhaseImplement)), "prepare", 2},
		{"branch ready, no staged plan", withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan))), "plan.write", 3},
		{"staged plan, design missing", withStagedPlan(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))), "plan.design", 3},
		{"design approved, DA missing", withDesign(withStagedPlan(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan))))), "plan.review", 3},
		{"DA stop unwaived", withDAStop(withDesign(withStagedPlan(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))))), "plan.review", 3},
		{"all planner gates recorded", withDA(withDesign(withStagedPlan(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))))), "plan.handoff", 3},
		{"orca claimable right after prepare", withClaimableLease(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))), "claim", 3},
		{"released lease in implement", withReleasedLease(withBranch(withIssue(baseRecord(issueopsnextcontract.PhaseImplement)))), "claim", 4},
		{"self holder still in plan phase", withSelfHolder(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))), "implement.enter", 4},
		{"self holder in implement", withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement)), "implement", 4},
		{"cleanup not recorded", withCompletionMissing(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean)), issueopsnextcontract.PhaseAISlopClean, "ai_slop_clean_at"), "clean", 5},
		{"cleanup recorded docs review missing", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean)), "project_docs_review"), "docs", 6},
		{"docs reviewed implementation review missing", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean)), "implementation_review"), "verify", 7},
		{"feedback contract update missing", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseFeedback)), "contract_feedback_issue_update"), "verify", 7},
		{"stale seal returns to clean", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean)), "ai_slop_clean_stale"), "clean", 5},
		{"gates ledger incomplete", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean)), "gates_incomplete:.issueops/gates.md"), "verify", 7},
		{"only upstream missing", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean)), "upstream"), "commit-push", 8},
		{"pushed and clean", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean))), "commit-push", 8},
		{"unmatched local key falls back", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseFeedback)), "branch_match"), "unknown", 7},
		{"pr without artifact", withSelfHolder(baseRecord(issueopsnextcontract.PhasePR)), "pr.create", 9},
		{"pr with artifact", withArtifact(withSelfHolder(baseRecord(issueopsnextcontract.PhasePR))), "pr.complete", 9},
		{"pr without execution", withIssue(baseRecord(issueopsnextcontract.PhasePR)), "unknown", 9},
		{"done", baseRecord(issueopsnextcontract.PhaseDone), "done", 10},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := Classify(testCase.in)
			if got.Stage.Key != testCase.key || got.Stage.Index != testCase.idx {
				t.Fatalf("got %s/%d want %s/%d", got.Stage.Key, got.Stage.Index, testCase.key, testCase.idx)
			}
		})
	}
}

// 규칙 순서가 뒤집히면 done 사이클이 미해결 pending을 숨긴다. 규칙 2가 규칙
// 21보다 먼저여야 한다.
func TestClassifyChecksPendingBeforePhase(t *testing.T) {
	got := Classify(withPending(baseRecord(issueopsnextcontract.PhaseDone)))
	if got.Stage.Key != "blocked.pending" {
		t.Fatalf("pending intent must win over the done phase, got %s", got.Stage.Key)
	}
	if !strings.Contains(got.NextCommand, "execution reconcile") {
		t.Fatalf("pending intent must route to reconcile, got %q", got.NextCommand)
	}
}

func TestClassifyNextCommandTable(t *testing.T) {
	cases := []struct {
		name   string
		in     Input
		prefix string
		kind   string
	}{
		{"none starts a cycle", Input{SourceRoot: "/repo"}, "issueops start --repo /repo", "exact"},
		{"issue records intent", baseRecord(issueopsnextcontract.PhaseGrill), "issueops intent record", "template"},
		{"handoff prepares execution", withDA(withDesign(withStagedPlan(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))))), "issueops execution prepare", "template"},
		{"stop verdict regresses", withDAStop(withDesign(withStagedPlan(withBranch(withIssue(baseRecord(issueopsnextcontract.PhasePlan)))))), "issueops regress --id io-test", "template"},
		{"commit-push checks strict readiness", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseAISlopClean)), "upstream"), "issueops pr-readiness --id io-test --strict", "exact"},
		{"done checks cleanup", baseRecord(issueopsnextcontract.PhaseDone), "issueops cleanup status --id io-test --merged", "exact"},
		{"unknown falls back to status", withLocal(withSelfHolder(baseRecord(issueopsnextcontract.PhaseFeedback)), "branch_match"), "issueops status --id io-test", "exact"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := Classify(testCase.in)
			if !strings.HasPrefix(got.NextCommand, testCase.prefix) {
				t.Fatalf("next command %q must start with %q", got.NextCommand, testCase.prefix)
			}
			if got.NextCommandKind != testCase.kind {
				t.Fatalf("next command kind %q want %q", got.NextCommandKind, testCase.kind)
			}
		})
	}
}

// 1단계는 missing을 알파벳 순으로 쓰면 안 된다. branch는 2단계 항목이다.
func TestClassifyIssueStageNeverRecommendsBranchPrepare(t *testing.T) {
	got := Classify(baseRecord(issueopsnextcontract.PhaseGrill))
	if strings.Contains(got.NextCommand, "branch prepare") {
		t.Fatalf("stage 1 must not recommend branch prepare, got %q", got.NextCommand)
	}
	if len(got.Missing) == 0 || got.Missing[0] != "intent_contract" {
		t.Fatalf("stage 1 missing must lead with intent_contract, got %v", got.Missing)
	}
}

func TestClassifyExits(t *testing.T) {
	self := Classify(withSelfHolder(baseRecord(issueopsnextcontract.PhaseImplement)))
	if !strings.HasPrefix(self.Exits.PauseCommand, "issueops execution release") {
		t.Fatalf("self holder must be able to pause, got %q", self.Exits.PauseCommand)
	}
	stale := Classify(withHolder(baseRecord(issueopsnextcontract.PhaseImplement), "codex", "s-2", false))
	if !strings.HasPrefix(stale.Exits.TakeoverCommand, "issueops execution replace") {
		t.Fatalf("stale holder must be replaceable, got %q", stale.Exits.TakeoverCommand)
	}
	if !strings.HasPrefix(self.Exits.AbandonCommand, "issueops cleanup abandon --id io-test --reason") {
		t.Fatalf("abandon must always be offered, got %q", self.Exits.AbandonCommand)
	}
	none := Classify(Input{})
	if none.Exits.AbandonCommand != "" {
		t.Fatalf("without a cycle there is nothing to abandon, got %q", none.Exits.AbandonCommand)
	}
}

// 관측하지 못한 홀더는 살아 있는 것으로 본다. 확인하지 않은 세션의 lease를
// 빼앗으라고 권하면 두 세션이 같은 워크트리를 쓴다.
func TestClassifyTreatsUnobservedHolderAsLive(t *testing.T) {
	in := withHolder(baseRecord(issueopsnextcontract.PhaseImplement), "codex", "s-2", true)
	in.HolderLive = nil
	if got := Classify(in); got.Stage.Key != "blocked.holder_live" {
		t.Fatalf("an unobserved holder must block, got %s", got.Stage.Key)
	}
}

func TestOwnerCommandCoversGateMap(t *testing.T) {
	keys := []string{
		"intent_contract", "raw_request", "interpreted_intent", "success_criteria",
		"plan_prep_decisions", "plan_prep_related_issues", "plan_prep_web_research", "plan_prep_codebase_survey",
		"issue_url", "branch", "branch_prepare", "split_decision", "domain_review",
		"design_review", "design_open_questions", "compatibility_review", "compatibility_blockers",
		"devils_advocate_review", "devils_advocate_review_stale",
		"worktree_path", "worktree_exists", "plan_artifact", "plan_path", "plan_exists", "plan_in_worktree",
		"execution", "execution_valid", "execution_worktree_match", "execution_write_lease",
		"implementation_changes", "ai_slop_clean_at", "ai_slop_clean_head", "ai_slop_clean_fingerprint",
		"ai_slop_clean_stale", "cleanup_evidence", "verification_evidence", "current_fingerprint",
		"implementation_review", "implementation_review_stale",
		"project_docs_review", "project_docs_review_stale",
		"schema_evidence", "schema_evidence_stale",
		"feedback_classification", "feedback_resolution", "contract_feedback_issue_update",
		"remote_artifact", "child_incomplete", "child_unvalidated", "child_rejected_unresolved",
		"gates_incomplete:.issueops/gates.md", "duplicate_issue_artifact:https://example.com/1",
		"worktree_clean", "upstream", "upstream_fetch", "upstream_synced", "branch_match",
	}
	for _, key := range keys {
		command := OwnerCommand("io-test", key)
		if strings.TrimSpace(command) == "" {
			t.Fatalf("missing owner command for %q", key)
		}
		if !strings.HasPrefix(command, "issueops ") {
			t.Fatalf("owner command for %q must be an issueops command, got %q", key, command)
		}
	}
	if got := OwnerCommand("io-test", "who_knows"); got != "issueops status --id io-test --json" {
		t.Fatalf("an unknown gate must fall back to status, got %q", got)
	}
}
