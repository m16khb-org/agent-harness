package mcpcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
)

func TestMCPIssueOpsChildLifecycle(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	parent := startIssueOpsMCPReadyDelegationParent(t, filepath.Join(t.TempDir(), "delegation-mcp"), "123-parent-child-mcp")

	startedOutcome := handleIssueOpsMCPToolCall(MCPToolCall{Name: "issueops_child_start", Arguments: map[string]any{
		"parent":     parent.ID,
		"branch":     "123-child-mcp",
		"title":      "child mcp task",
		"scope":      "implement the delegated child mcp fixture",
		"acceptance": []any{"child mcp start returns JSON", "parent can validate child result"},
	}})
	if startedOutcome.Err != nil {
		t.Fatalf("child start outcome error: %#v", startedOutcome.Err)
	}
	started, ok := startedOutcome.Payload.(core.IssueOpsChildStartResult)
	if !ok {
		t.Fatalf("child start payload type = %T", startedOutcome.Payload)
	}
	if !started.OK || started.ParentID != parent.ID || started.Child.ID == "" || started.ParentRef.CycleID != started.Child.ID {
		t.Fatalf("unexpected child start result: %#v", started)
	}
	if !strings.Contains(started.Guidance, parent.Branch) || !strings.Contains(started.Guidance, "HARNESS_EXPECTED_WORKTREE") {
		t.Fatalf("child start should surface core delegation guidance, got %q", started.Guidance)
	}

	statusOutcome := handleIssueOpsMCPToolCall(MCPToolCall{Name: "issueops_child_status", Arguments: map[string]any{"parent": parent.ID}})
	if statusOutcome.Err != nil {
		t.Fatalf("child status outcome error: %#v", statusOutcome.Err)
	}
	status, ok := statusOutcome.Payload.(core.IssueOpsChildStatusResult)
	if !ok || !status.OK || len(status.Children) != 1 || status.Children[0].CycleID != started.Child.ID {
		t.Fatalf("child status should include started child: %#v", statusOutcome.Payload)
	}

	child := started.Child
	child.Phase = core.IssueOpsPhaseDone
	if _, err := core.WriteIssueOps(core.IssueOpsStateRoot(), child); err != nil {
		t.Fatal(err)
	}

	acceptedOutcome := handleIssueOpsMCPToolCall(MCPToolCall{Name: "issueops_child_accept", Arguments: map[string]any{
		"parent":   parent.ID,
		"child":    child.ID,
		"evidence": []any{"parent verified child output"},
	}})
	if acceptedOutcome.Err != nil {
		t.Fatalf("child accept outcome error: %#v", acceptedOutcome.Err)
	}
	accepted, ok := acceptedOutcome.Payload.(core.IssueOpsChildValidationResult)
	if !ok || !accepted.OK || accepted.ParentRef.ValidationVerdict != "accepted" || len(accepted.ParentRef.ValidationEvidence) != 1 {
		t.Fatalf("child accept should record accepted verdict and evidence: %#v", acceptedOutcome.Payload)
	}

	rejectedOutcome := handleIssueOpsMCPToolCall(MCPToolCall{Name: "issueops_child_reject", Arguments: map[string]any{
		"parent": parent.ID,
		"child":  child.ID,
		"reason": "needs a clearer verification artifact",
	}})
	if rejectedOutcome.Err != nil {
		t.Fatalf("child reject outcome error: %#v", rejectedOutcome.Err)
	}
	rejected, ok := rejectedOutcome.Payload.(core.IssueOpsChildValidationResult)
	if !ok || rejected.ParentRef.ValidationVerdict != "rejected" || !strings.Contains(rejected.ParentRef.ValidationReason, "clearer verification") {
		t.Fatalf("child reject should record rejected verdict and reason: %#v", rejectedOutcome.Payload)
	}

	droppedOutcome := handleIssueOpsMCPToolCall(MCPToolCall{Name: "issueops_child_drop", Arguments: map[string]any{
		"parent": parent.ID,
		"child":  child.ID,
		"reason": "scope deliberately removed from parent plan",
	}})
	if droppedOutcome.Err != nil {
		t.Fatalf("child drop outcome error: %#v", droppedOutcome.Err)
	}
	dropped, ok := droppedOutcome.Payload.(core.IssueOpsChildValidationResult)
	if !ok || dropped.ParentRef.ValidationVerdict != "dropped" || !strings.Contains(dropped.ParentRef.ValidationReason, "removed from parent") {
		t.Fatalf("child drop should record dropped verdict and reason: %#v", droppedOutcome.Payload)
	}

	if errOutcome := handleIssueOpsMCPToolCall(MCPToolCall{Name: "issueops_child_status", Arguments: map[string]any{}}); !errOutcome.IsError {
		t.Fatalf("missing parent should be a normalized error result, got %#v", errOutcome)
	}
	if errOutcome := handleIssueOpsMCPToolCall(MCPToolCall{Name: "issueops_child_reject", Arguments: map[string]any{"parent": parent.ID, "child": child.ID, "reason": "short"}}); !errOutcome.IsError {
		t.Fatalf("short reject reason should be a normalized error result, got %#v", errOutcome)
	}
}

func startIssueOpsMCPReadyDelegationParent(t *testing.T, repo, branch string) core.IssueOpsRecord {
	t.Helper()
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatalf("StartIssueOps: %v", err)
	}
	worktree := makeIssueOpsMCPWorktreeForDelegationTest(t, repo, branch)
	planPath := filepath.Join(worktree, "plans", "parent.md")
	writeIssueOpsMCPFileForTest(t, worktree, "plans/parent.md", "parent plan\n")
	prepareIssueOpsMCPParentImplementationSurface(t, record.ID, branch, worktree)
	recordIssueOpsMCPParentDelegationPrereqs(t, record.ID, planPath)
	record, err = core.ReadIssueOps(core.IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = core.IssueOpsPhaseImplement
	record, err = core.WriteIssueOps(core.IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func makeIssueOpsMCPWorktreeForDelegationTest(t *testing.T, repo, slug string) string {
	t.Helper()
	worktree := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", slug)
	if err := os.MkdirAll(filepath.Join(worktree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git", "HEAD"), []byte("ref: refs/heads/"+slug+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return worktree
}

func writeIssueOpsMCPFileForTest(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func prepareIssueOpsMCPParentImplementationSurface(t *testing.T, id, branch, worktree string) {
	t.Helper()
	if _, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), id, "https://github.com/example/repo/issues/123"); err != nil {
		t.Fatal(err)
	}
	if _, err := core.PrepareIssueOpsBranch(core.IssueOpsStateRoot(), id, core.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     "https://github.com/example/repo/issues/123",
		Branch:       branch,
		BaseBranch:   "main",
		LinkVerified: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := core.LinkIssueOpsWorktree(core.IssueOpsStateRoot(), id, worktree); err != nil {
		t.Fatal(err)
	}
}

func recordIssueOpsMCPParentDelegationPrereqs(t *testing.T, id, planPath string) {
	t.Helper()
	if _, err := core.RecordIssueOpsIntent(core.IssueOpsStateRoot(), id, core.IssueOpsIntentRecordRequest{
		RawRequest:        "delegate issueops child work",
		InterpretedIntent: "keep delegation MCP tools aligned with CLI JSON shapes",
		SuccessCriteria:   []string{"child starts through MCP", "parent validates child through MCP"},
	}); err != nil {
		t.Fatal(err)
	}
	waived := core.IssueOpsPlanPrepItemRequest{WaiveReason: "mcp lifecycle test"}
	if _, err := core.RecordIssueOpsPlanPrep(core.IssueOpsStateRoot(), id, core.IssueOpsPlanPrepRequest{
		PriorDecisions: waived,
		RelatedIssues:  waived,
		WebResearch:    waived,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := core.RecordIssueOpsDesignReview(core.IssueOpsStateRoot(), id, core.IssueOpsDesignReviewRequest{
		ProblemSummary: "IssueOps must preserve delegated child contracts",
		ProposedDesign: "Expose MCP tools as thin adapters over core child lifecycle APIs",
		RefactorPlan:   "Keep MCP registration and handlers scoped to child delegation tools",
		Alternatives:   []string{"CLI-only delegation"},
		Risks:          []string{"MCP schema drift from CLI JSON shape"},
		Verification:   []string{"design review checked alternatives and risks", "go test ./cmd/harness/mcpcli"},
		Approved:       true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := core.LinkIssueOpsPlan(core.IssueOpsStateRoot(), id, planPath); err != nil {
		t.Fatal(err)
	}
	if _, err := core.RecordIssueOpsDomainReview(core.IssueOpsStateRoot(), id, core.IssueOpsDomainReviewRequest{
		ModelFit:    "delegation mcp fixture follows IssueOps domain model",
		Terminology: []string{"parent cycle", "child cycle"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := core.RecordIssueOpsCompatibilityReview(core.IssueOpsStateRoot(), id, core.IssueOpsCompatibilityReviewRequest{
		BackwardCompatibility: []string{"existing IssueOps records remain readable"},
		SideEffects:           []string{"child MCP tools write only IssueOps state"},
		RollbackPlan:          "Revert child MCP tool registration.",
		Verification:          []string{"go test ./cmd/harness/mcpcli"},
		Approved:              true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := core.RecordIssueOpsDevilsAdvocateReview(core.IssueOpsStateRoot(), id, core.IssueOpsDevilsAdvocateReviewRequest{Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
	if _, err := core.RecordIssueOpsExecutionDecision(core.IssueOpsStateRoot(), id, core.IssueOpsExecutionDecisionRecordRequest{
		AutoProceed:       []string{"delegate child cycles when parent implement prerequisites are met"},
		HookBlocked:       []string{"hooks do not spawn sub-agents"},
		HumanGates:        []string{"ask before destructive cleanup"},
		SubagentUse:       "planned",
		SubagentRationale: "parent owns fan-out coordination",
		SubagentPlans: []core.IssueOpsSubAgentPlan{{
			Objective:            "delegate isolated child cycle",
			Pattern:              "task-fan-out-coordination",
			Benefit:              "parallel_speed",
			Tradeoffs:            []string{"parent must validate each result"},
			NetPositiveRationale: "fan-out is bounded and parent-owned",
			Scope:                "child branch and worktree only",
			Verification:         "parent accepts child with evidence",
			Fallback:             "main agent completes task directly",
		}},
	}); err != nil {
		t.Fatal(err)
	}
}
