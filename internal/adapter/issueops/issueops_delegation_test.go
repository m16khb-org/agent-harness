package issueops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"issueops/internal/contract/issueops"
)

func TestStartIssueOpsChildFailClosedPreconditions(t *testing.T) {
	cases := []struct {
		name        string
		childBranch string
		mutate      func(*testing.T, string, issueops.IssueOpsRecord) issueops.IssueOpsRecord
		missing     string
	}{
		{
			name:        "parent in plan phase",
			childBranch: "124-child-plan",
			mutate: func(t *testing.T, stateRoot string, parent issueops.IssueOpsRecord) issueops.IssueOpsRecord {
				t.Helper()
				parent.Phase = IssueOpsPhasePlan
				return writeIssueOpsRecordForDelegationTest(t, stateRoot, parent)
			},
			missing: "parent_phase_not_implement",
		},
		{
			name:        "unapproved design review",
			childBranch: "124-child-design",
			mutate: func(t *testing.T, stateRoot string, parent issueops.IssueOpsRecord) issueops.IssueOpsRecord {
				t.Helper()
				parent.DesignReview.Approved = false
				return writeIssueOpsRecordForDelegationTest(t, stateRoot, parent)
			},
			missing: "parent_design_review_unapproved",
		},
		{
			name:        "blocked compatibility review",
			childBranch: "124-child-compat",
			mutate: func(t *testing.T, stateRoot string, parent issueops.IssueOpsRecord) issueops.IssueOpsRecord {
				t.Helper()
				parent.CompatibilityReview.Approved = false
				parent.CompatibilityReview.Blockers = []string{"open compatibility blocker"}
				return writeIssueOpsRecordForDelegationTest(t, stateRoot, parent)
			},
			missing: "parent_compatibility_unapproved",
		},
		{
			name:        "missing devils advocate review",
			childBranch: "124-child-da",
			mutate: func(t *testing.T, stateRoot string, parent issueops.IssueOpsRecord) issueops.IssueOpsRecord {
				t.Helper()
				parent.DevilsAdvocateReview = nil
				return writeIssueOpsRecordForDelegationTest(t, stateRoot, parent)
			},
			missing: "parent_devils_advocate_missing",
		},
		{
			name:        "parent already delegated child",
			childBranch: "124-child-depth",
			mutate: func(t *testing.T, stateRoot string, parent issueops.IssueOpsRecord) issueops.IssueOpsRecord {
				t.Helper()
				parent.Delegation = &issueops.IssueOpsDelegationContract{ParentCycleID: "io-root", TaskScope: "nested", DelegatedAt: "2026-07-07T00:00:00Z"}
				return writeIssueOpsRecordForDelegationTest(t, stateRoot, parent)
			},
			missing: "delegation_depth_exceeded",
		},
		{
			name:        "child branch equals parent",
			childBranch: "123-parent",
			mutate: func(t *testing.T, stateRoot string, parent issueops.IssueOpsRecord) issueops.IssueOpsRecord {
				return parent
			},
			missing: "child_branch_equals_parent",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			parent := createDelegationReadyParentForTest(t, stateRoot)
			parent = tc.mutate(t, stateRoot, parent)
			_, err := startIssueOpsChildForTest(stateRoot, parent, issueops.IssueOpsChildStartRequest{
				ParentID:           parent.ID,
				Branch:             tc.childBranch,
				Title:              "delegated child",
				TaskScope:          "handle delegated child work",
				AcceptanceCriteria: []string{"child work is isolated"},
			})
			if err == nil || !strings.Contains(err.Error(), tc.missing) {
				t.Fatalf("expected missing key %q, got %v", tc.missing, err)
			}
			childID := NewIssueOpsID(parent.Repo, tc.childBranch)
			if childID != parent.ID {
				if _, readErr := ReadIssueOps(stateRoot, childID); readErr == nil {
					t.Fatalf("blocked child start must not create child record %s", childID)
				}
			}
		})
	}
}

func TestStartIssueOpsChildCreatesDelegatedProfile(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	result, err := startIssueOpsChildForTest(stateRoot, parent, issueops.IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "123-child-profile",
		Title:              "child profile",
		TaskScope:          "implement the delegated profile",
		AcceptanceCriteria: []string{"delegated artifacts exist", "child earns own worktree gates"},
		ParentPlanPath:     parent.PlanPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	child := result.Child
	if child.Delegation == nil || child.Delegation.ParentCycleID != parent.ID {
		t.Fatalf("child delegation contract missing parent: %#v", child.Delegation)
	}
	if child.Intent == nil || child.Intent.IntentClass != "delegated-child" || child.Intent.InterpretedIntent != "implement the delegated profile" {
		t.Fatalf("child intent should be delegated from scope: %#v", child.Intent)
	}
	if child.IssueURL != parent.IssueURL {
		t.Fatalf("child should fall back to parent issue URL, got %q", child.IssueURL)
	}
	if child.PlanPrep == nil || child.PlanPrep.PriorDecisions.WaiveReason != "delegated:"+parent.ID {
		t.Fatalf("child plan prep should be delegated/waived: %#v", child.PlanPrep)
	}
	if child.DesignReview == nil || !child.DesignReview.Approved || child.DesignReview.RefactorPlan == "" || len(child.DesignReview.Alternatives) == 0 || len(child.DesignReview.Risks) == 0 {
		t.Fatalf("child design review should be approved and populated: %#v", child.DesignReview)
	}
	if child.CompatibilityReview == nil || !child.CompatibilityReview.Approved || len(child.CompatibilityReview.Verification) == 0 {
		t.Fatalf("child compatibility review should be approved and populated: %#v", child.CompatibilityReview)
	}
	if child.DevilsAdvocateReview == nil || !child.DevilsAdvocateReview.Waived || child.DevilsAdvocateReview.WaiverRationale != "delegated:"+parent.ID+" parent DA verdict pass" {
		t.Fatalf("child devil's-advocate review should be waived from parent pass: %#v", child.DevilsAdvocateReview)
	}
	if result.ParentRef.CycleID != child.ID || result.ParentRef.Branch != child.Branch || result.ParentRef.Title != "child profile" {
		t.Fatalf("parent ref should describe child: %#v", result.ParentRef)
	}
	if !strings.Contains(result.Guidance, parent.Branch) {
		t.Fatalf("guidance should name parent branch as child base, got %q", result.Guidance)
	}

	if child, err = AdvanceIssueOpsPhase(stateRoot, child.ID, string(IssueOpsPhaseGrill)); err != nil {
		t.Fatalf("child should enter grill from delegated problem artifacts: %v", err)
	}
	if child, err = AdvanceIssueOpsPhase(stateRoot, child.ID, string(IssueOpsPhasePlan)); err != nil {
		t.Fatalf("child should enter plan from delegated grill artifacts: %v", err)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, child.ID, string(IssueOpsPhaseCompatibilityReview)); err == nil || !strings.Contains(err.Error(), "branch_prepare") || !strings.Contains(err.Error(), "worktree_path") || !strings.Contains(err.Error(), "plan_path") {
		t.Fatalf("child compatibility-review should still require own branch/worktree/plan gates, got %v", err)
	}

	childWorktree := makeIssueOpsWorktreeDirForTest(t, parent.Repo, "123-child-profile")
	if _, err := PrepareIssueOpsBranch(stateRoot, child.ID, issueops.IssueOpsBranchPrepareRequest{Provider: "github", IssueURL: child.IssueURL, Branch: child.Branch, BaseBranch: parent.Branch, LinkVerified: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := LinkIssueOpsWorktree(stateRoot, child.ID, childWorktree); err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, childWorktree, "plans/child.md", "child plan\n")
	if _, err := LinkIssueOpsPlan(stateRoot, child.ID, filepath.Join(childWorktree, "plans/child.md")); err != nil {
		t.Fatal(err)
	}
	if child, err = AdvanceIssueOpsPhase(stateRoot, child.ID, string(IssueOpsPhaseCompatibilityReview)); err != nil {
		t.Fatalf("child should enter compatibility-review after earning own setup gates: %v", err)
	}
	if _, err := AdvanceIssueOpsPhase(stateRoot, child.ID, string(IssueOpsPhaseImplement)); err == nil || !strings.Contains(err.Error(), "execution") || strings.Contains(err.Error(), "devils_advocate") {
		t.Fatalf("child implement should require only its own execution lease (the synthesized parent verdict is exempt from plan binding), got %v", err)
	}
}

func TestStartIssueOpsChildPerConditionRemedy(t *testing.T) {
	cases := []struct {
		name          string
		childBranch   string
		missing       string
		breakParent   func(issueops.IssueOpsRecord) issueops.IssueOpsRecord
		remedy        func(issueops.IssueOpsRecord, *issueops.IssueOpsChildStartRequest) issueops.IssueOpsRecord
		remedyMessage string
	}{
		{
			name:        "parent phase",
			childBranch: "123-child-remedy-phase",
			missing:     "parent_phase_not_implement",
			breakParent: func(parent issueops.IssueOpsRecord) issueops.IssueOpsRecord {
				parent.Phase = IssueOpsPhasePlan
				return parent
			},
			remedy: func(parent issueops.IssueOpsRecord, req *issueops.IssueOpsChildStartRequest) issueops.IssueOpsRecord {
				parent.Phase = IssueOpsPhaseImplement
				return parent
			},
			remedyMessage: "fixing only parent phase",
		},
		{
			name:        "design review",
			childBranch: "123-child-remedy-design",
			missing:     "parent_design_review_unapproved",
			breakParent: func(parent issueops.IssueOpsRecord) issueops.IssueOpsRecord {
				parent.DesignReview.Approved = false
				return parent
			},
			remedy: func(parent issueops.IssueOpsRecord, req *issueops.IssueOpsChildStartRequest) issueops.IssueOpsRecord {
				parent.DesignReview.Approved = true
				return parent
			},
			remedyMessage: "fixing only design approval",
		},
		{
			name:        "compatibility review",
			childBranch: "123-child-remedy-compat",
			missing:     "parent_compatibility_unapproved",
			breakParent: func(parent issueops.IssueOpsRecord) issueops.IssueOpsRecord {
				parent.CompatibilityReview.Approved = false
				parent.CompatibilityReview.Blockers = []string{"open compatibility blocker"}
				return parent
			},
			remedy: func(parent issueops.IssueOpsRecord, req *issueops.IssueOpsChildStartRequest) issueops.IssueOpsRecord {
				parent.CompatibilityReview.Approved = true
				parent.CompatibilityReview.Blockers = nil
				return parent
			},
			remedyMessage: "fixing only compatibility approval",
		},
		{
			name:        "devils advocate review",
			childBranch: "123-child-remedy-da",
			missing:     "parent_devils_advocate_missing",
			breakParent: func(parent issueops.IssueOpsRecord) issueops.IssueOpsRecord {
				parent.DevilsAdvocateReview = nil
				return parent
			},
			remedy: func(parent issueops.IssueOpsRecord, req *issueops.IssueOpsChildStartRequest) issueops.IssueOpsRecord {
				parent.DevilsAdvocateReview = &issueops.IssueOpsDevilsAdvocateReview{Verdict: "pass", RecordedAt: "2026-07-07T00:00:00Z"}
				return parent
			},
			remedyMessage: "fixing only devil's-advocate review",
		},
		{
			name:        "delegation depth",
			childBranch: "123-child-remedy-depth",
			missing:     "delegation_depth_exceeded",
			breakParent: func(parent issueops.IssueOpsRecord) issueops.IssueOpsRecord {
				parent.Delegation = &issueops.IssueOpsDelegationContract{ParentCycleID: "io-root", TaskScope: "nested", DelegatedAt: "2026-07-07T00:00:00Z"}
				return parent
			},
			remedy: func(parent issueops.IssueOpsRecord, req *issueops.IssueOpsChildStartRequest) issueops.IssueOpsRecord {
				parent.Delegation = nil
				return parent
			},
			remedyMessage: "fixing only delegation depth",
		},
		{
			name:        "child branch",
			childBranch: "123-parent",
			missing:     "child_branch_equals_parent",
			breakParent: func(parent issueops.IssueOpsRecord) issueops.IssueOpsRecord {
				return parent
			},
			remedy: func(parent issueops.IssueOpsRecord, req *issueops.IssueOpsChildStartRequest) issueops.IssueOpsRecord {
				req.Branch = "123-child-remedy-branch"
				return parent
			},
			remedyMessage: "fixing only the child branch",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			parent := createDelegationReadyParentForTest(t, stateRoot)
			req := issueops.IssueOpsChildStartRequest{
				ParentID:           parent.ID,
				Branch:             tc.childBranch,
				Title:              "child remedy",
				TaskScope:          "prove a single precondition remedy",
				AcceptanceCriteria: []string{"single field remedy succeeds"},
			}
			parent = writeIssueOpsRecordForDelegationTest(t, stateRoot, tc.breakParent(parent))
			if _, err := startIssueOpsChildForTest(stateRoot, parent, req); err == nil || !strings.Contains(err.Error(), tc.missing) {
				t.Fatalf("expected %s precondition failure, got %v", tc.missing, err)
			}
			childID := NewIssueOpsID(parent.Repo, req.Branch)
			if childID != parent.ID {
				if _, readErr := ReadIssueOps(stateRoot, childID); readErr == nil {
					t.Fatalf("blocked child start must not create child record %s", childID)
				}
			}
			parent = writeIssueOpsRecordForDelegationTest(t, stateRoot, tc.remedy(parent, &req))
			if _, err := startIssueOpsChildForTest(stateRoot, parent, req); err != nil {
				t.Fatalf("%s should unblock child start: %v", tc.remedyMessage, err)
			}
		})
	}
}

func TestStartIssueOpsChildLinkFailureReturnsWarning(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	result, err := startIssueOpsChildForTest(stateRoot, parent, issueops.IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "124-child-cross-project",
		Title:              "cross project child",
		TaskScope:          "preserve child despite remote link warning",
		AcceptanceCriteria: []string{"child exists", "parent ref exists"},
		ChildIssueURL:      "https://github.com/other/repo/issues/124",
	})
	if err != nil {
		t.Fatalf("remote child-link failure should downgrade to warning: %v", err)
	}
	if result.ChildLinkWarning == "" {
		t.Fatalf("expected child_link_warning for cross-project child link: %#v", result)
	}
	if _, err := ReadIssueOps(stateRoot, result.Child.ID); err != nil {
		t.Fatalf("child should remain created after link warning: %v", err)
	}
	parentAfter, err := ReadIssueOps(stateRoot, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !hasChildRef(parentAfter.ChildCycles, result.Child.ID) {
		t.Fatalf("parent ref should remain after link warning: %#v", parentAfter.ChildCycles)
	}
}

func TestStaleResetPreservesDelegationGraph(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	result, err := startIssueOpsChildForTest(stateRoot, parent, issueops.IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "123-child-stale",
		Title:              "stale child",
		TaskScope:          "preserve delegation across stale reset",
		AcceptanceCriteria: []string{"delegation graph survives stale reset"},
	})
	if err != nil {
		t.Fatal(err)
	}
	parent, err = ReadIssueOps(stateRoot, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	parentWorktree := parent.WorktreePath
	if err := os.RemoveAll(parentWorktree); err != nil {
		t.Fatal(err)
	}
	resetParent, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: parent.Repo, Branch: parent.Branch})
	if err != nil {
		t.Fatal(err)
	}
	if !hasChildRef(resetParent.ChildCycles, result.Child.ID) {
		t.Fatalf("parent stale reset should preserve child refs: %#v", resetParent.ChildCycles)
	}

	child := result.Child
	childWorktree := makeIssueOpsWorktreeDirForTest(t, parent.Repo, "123-child-stale")
	child.Phase = IssueOpsPhaseImplement
	child.WorktreePath = childWorktree
	writeIssueOpsRecordForDelegationTest(t, stateRoot, child)
	if err := os.RemoveAll(childWorktree); err != nil {
		t.Fatal(err)
	}
	resetChild, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: child.Repo, Branch: child.Branch})
	if err != nil {
		t.Fatal(err)
	}
	if resetChild.Delegation == nil || resetChild.Delegation.ParentCycleID != parent.ID {
		t.Fatalf("child stale reset should preserve delegation pointer: %#v", resetChild.Delegation)
	}
}

func TestStartIssueOpsChildAppendsParentRef(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	req := issueops.IssueOpsChildStartRequest{
		ParentID:           parent.ID,
		Branch:             "124-child-link",
		Title:              "linked child",
		TaskScope:          "append parent ref and remote link",
		AcceptanceCriteria: []string{"parent ref dedupes", "remote child link is recorded"},
		ChildIssueURL:      "https://github.com/example/repo/issues/124",
	}
	result, err := startIssueOpsChildForTest(stateRoot, parent, req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := startIssueOpsChildForTest(stateRoot, parent, req)
	if err != nil {
		t.Fatal(err)
	}
	if second.Child.ID != result.Child.ID {
		t.Fatalf("same branch should resolve same child id, got %s then %s", result.Child.ID, second.Child.ID)
	}
	parentAfter, err := ReadIssueOps(stateRoot, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if countChildRefs(parentAfter.ChildCycles, result.Child.ID) != 1 {
		t.Fatalf("parent ref should be deduped, got %#v", parentAfter.ChildCycles)
	}
	foundRemote := false
	for _, link := range parentAfter.IssueLinks {
		if link.Type == "child" && link.URL == req.ChildIssueURL {
			foundRemote = true
		}
	}
	if !foundRemote {
		t.Fatalf("child issue URL should be linked remotely on parent: %#v", parentAfter.IssueLinks)
	}
}

func TestAppendIssueOpsChildRefResetsTerminalReceiptOnlyForNewerIncarnation(t *testing.T) {
	cases := []struct {
		name             string
		childCreatedAt   func(time.Time, string) string
		wantReceiptReset bool
	}{
		{
			name: "newer child incarnation clears terminal receipt",
			childCreatedAt: func(existing time.Time, _ string) string {
				return existing.Add(time.Nanosecond).Format(time.RFC3339Nano)
			},
			wantReceiptReset: true,
		},
		{
			name: "same live child replay retains terminal receipt",
			childCreatedAt: func(existing time.Time, _ string) string {
				return existing.Format(time.RFC3339Nano)
			},
		},
		{
			name: "malformed child timestamp retains terminal receipt",
			childCreatedAt: func(_ time.Time, _ string) string {
				return "not-a-timestamp"
			},
		},
		{
			name: "older child timestamp retains terminal receipt",
			childCreatedAt: func(existing time.Time, _ string) string {
				return existing.Add(-time.Nanosecond).Format(time.RFC3339Nano)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			parent := createDelegationReadyParentForTest(t, stateRoot)
			req := issueops.IssueOpsChildStartRequest{
				ParentID:           parent.ID,
				Branch:             "124-child-incarnation",
				Title:              "incarnation child",
				TaskScope:          "preserve parent receipt unless a new child record exists",
				AcceptanceCriteria: []string{"newer records reset terminal validation only"},
			}
			started, err := startIssueOpsChildForTest(stateRoot, parent, req)
			if err != nil {
				t.Fatal(err)
			}

			parent, err = ReadIssueOps(stateRoot, parent.ID)
			if err != nil {
				t.Fatal(err)
			}
			existing, ok := childRefByID(parent.ChildCycles, started.Child.ID)
			if !ok {
				t.Fatalf("missing child ref: %#v", parent.ChildCycles)
			}
			existingTime, err := time.Parse(time.RFC3339Nano, existing.CreatedAt)
			if err != nil {
				t.Fatalf("parse parent ref CreatedAt: %v", err)
			}
			for i := range parent.ChildCycles {
				if parent.ChildCycles[i].CycleID == started.Child.ID {
					parent.ChildCycles[i].ValidationVerdict = "dropped"
					parent.ChildCycles[i].ValidationReason = "reconciled external operation left no resource"
					parent.ChildCycles[i].ValidationEvidence = []string{"cleanup receipt"}
					parent.ChildCycles[i].ValidatedAt = existing.CreatedAt
				}
			}
			parent = writeIssueOpsRecordForDelegationTest(t, stateRoot, parent)

			child := started.Child
			child.CreatedAt = tc.childCreatedAt(existingTime, child.CreatedAt)
			child = writeIssueOpsRecordForDelegationTest(t, stateRoot, child)
			now := existingTime.Add(time.Minute).Format(time.RFC3339Nano)
			actor := issueOpsActorForTest(parent.WorktreePath)
			ref, err := appendIssueOpsChildRef(stateRoot, parent.ID, child, req, now, &actor)
			if err != nil {
				t.Fatal(err)
			}

			if tc.wantReceiptReset {
				if ref.ValidationVerdict != "" || ref.ValidationReason != "" || len(ref.ValidationEvidence) != 0 || ref.ValidatedAt != "" {
					t.Fatalf("new child incarnation should clear terminal receipt: %#v", ref)
				}
				status, err := IssueOpsChildStatus(stateRoot, parent.ID, false)
				if err != nil {
					t.Fatal(err)
				}
				if len(status.Children) != 1 || status.Children[0].Phase != IssueOpsPhaseProblem || !status.Children[0].Indexed || !status.Children[0].Scanned || status.Children[0].ValidationVerdict != "" {
					t.Fatalf("new incarnation should be active and incomplete in child status: %#v", status)
				}
				return
			}
			if ref.ValidationVerdict != "dropped" || ref.ValidationReason == "" || len(ref.ValidationEvidence) != 1 || ref.ValidatedAt == "" {
				t.Fatalf("non-new child record must retain terminal receipt: %#v", ref)
			}
		})
	}
}

func TestArchivedIssueOpsChildDoesNotOverwriteReappearedIncarnation(t *testing.T) {
	cases := []struct {
		name     string
		archived func(string, string, string, *IssueOpsActor) error
	}{
		{
			name: "accept",
			archived: func(stateRoot, parentID, childID string, actor *IssueOpsActor) error {
				_, err := acceptArchivedIssueOpsChild(stateRoot, parentID, childID, []string{"archived verification"}, actor)
				return err
			},
		},
		{
			name: "drop",
			archived: func(stateRoot, parentID, childID string, actor *IssueOpsActor) error {
				_, err := dropArchivedIssueOpsChild(stateRoot, parentID, childID, "archived child is absent", actor)
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			parent := createDelegationReadyParentForTest(t, stateRoot)
			req := issueops.IssueOpsChildStartRequest{
				ParentID:           parent.ID,
				Branch:             "124-archived-incarnation",
				Title:              "archived incarnation child",
				TaskScope:          "preserve a reappeared child's active parent ref",
				AcceptanceCriteria: []string{"archived receipt writes reject a reappeared child"},
			}
			started, err := startIssueOpsChildForTest(stateRoot, parent, req)
			if err != nil {
				t.Fatal(err)
			}

			parent, err = ReadIssueOps(stateRoot, parent.ID)
			if err != nil {
				t.Fatal(err)
			}
			existing, ok := childRefByID(parent.ChildCycles, started.Child.ID)
			if !ok {
				t.Fatalf("missing child ref: %#v", parent.ChildCycles)
			}
			existingTime, err := time.Parse(time.RFC3339Nano, existing.CreatedAt)
			if err != nil {
				t.Fatalf("parse parent ref CreatedAt: %v", err)
			}
			for i := range parent.ChildCycles {
				if parent.ChildCycles[i].CycleID == started.Child.ID {
					parent.ChildCycles[i].ValidationVerdict = "dropped"
					parent.ChildCycles[i].ValidationReason = "archived cleanup receipt"
					parent.ChildCycles[i].ValidatedAt = existing.CreatedAt
				}
			}
			parent = writeIssueOpsRecordForDelegationTest(t, stateRoot, parent)

			child := started.Child
			child.CreatedAt = existingTime.Add(time.Nanosecond).Format(time.RFC3339Nano)
			child = writeIssueOpsRecordForDelegationTest(t, stateRoot, child)
			actor := issueOpsActorForTest(parent.WorktreePath)
			if _, err := appendIssueOpsChildRef(stateRoot, parent.ID, child, req, existingTime.Add(time.Minute).Format(time.RFC3339Nano), &actor); err != nil {
				t.Fatal(err)
			}

			if err := tc.archived(stateRoot, parent.ID, child.ID, &actor); err == nil || !strings.Contains(err.Error(), "child_reappeared") {
				t.Fatalf("archived writer should reject a reappeared child, got %v", err)
			}
			parent, err = ReadIssueOps(stateRoot, parent.ID)
			if err != nil {
				t.Fatal(err)
			}
			ref, ok := childRefByID(parent.ChildCycles, child.ID)
			if !ok {
				t.Fatalf("missing child ref after archived writer: %#v", parent.ChildCycles)
			}
			if ref.ValidationVerdict != "" || ref.ValidationReason != "" || ref.ValidatedAt != "" {
				t.Fatalf("reappeared child ref must remain active and incomplete: %#v", ref)
			}
		})
	}
}

func TestArchivedIssueOpsChildRecordsReceiptWhenChildRemainsAbsent(t *testing.T) {
	cases := []struct {
		name     string
		verdict  string
		archived func(string, string, string, *IssueOpsActor) (issueops.IssueOpsChildValidationResult, error)
	}{
		{
			name:    "accept",
			verdict: "accepted",
			archived: func(stateRoot, parentID, childID string, actor *IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
				return acceptArchivedIssueOpsChild(stateRoot, parentID, childID, []string{"archived verification"}, actor)
			},
		},
		{
			name:    "drop",
			verdict: "dropped",
			archived: func(stateRoot, parentID, childID string, actor *IssueOpsActor) (issueops.IssueOpsChildValidationResult, error) {
				return dropArchivedIssueOpsChild(stateRoot, parentID, childID, "archived child is absent", actor)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			parent := createDelegationReadyParentForTest(t, stateRoot)
			req := issueops.IssueOpsChildStartRequest{
				ParentID:           parent.ID,
				Branch:             "124-archived-absence",
				Title:              "archived absence child",
				TaskScope:          "record an archived child receipt when no child remains",
				AcceptanceCriteria: []string{"archived child receipt remains supported"},
			}
			started, err := startIssueOpsChildForTest(stateRoot, parent, req)
			if err != nil {
				t.Fatal(err)
			}
			if err := deleteIssueOps(stateRoot, started.Child.ID); err != nil {
				t.Fatal(err)
			}

			actor := issueOpsActorForTest(parent.WorktreePath)
			result, err := tc.archived(stateRoot, parent.ID, started.Child.ID, &actor)
			if err != nil {
				t.Fatal(err)
			}
			if result.ParentRef.ValidationVerdict != tc.verdict || result.ParentRef.ValidatedAt == "" {
				t.Fatalf("archived child receipt mismatch: %#v", result.ParentRef)
			}
		})
	}
}

func TestStartIssueOpsChildConcurrentSameBranch(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	const workers = 2
	var wg sync.WaitGroup
	type outcome struct {
		title  string
		result issueops.IssueOpsChildStartResult
		err    error
	}
	outcomes := make(chan outcome, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			title := fmt.Sprintf("same child %d", i)
			result, err := startIssueOpsChildForTest(stateRoot, parent, issueops.IssueOpsChildStartRequest{
				ParentID:           parent.ID,
				Branch:             "123-child-same",
				Title:              title,
				TaskScope:          "same branch concurrency",
				AcceptanceCriteria: []string{"one child record"},
			})
			outcomes <- outcome{title: title, result: result, err: err}
		}(i)
	}
	wg.Wait()
	close(outcomes)
	requestedTitles := map[string]bool{}
	for outcome := range outcomes {
		requestedTitles[outcome.title] = true
		if outcome.err != nil {
			t.Fatalf("same-branch concurrent child start should be idempotent: %v", outcome.err)
		}
		if outcome.result.ParentRef.CycleID != outcome.result.Child.ID || outcome.result.ParentRef.Branch != "123-child-same" {
			t.Fatalf("same-branch result ref should match the child record: %#v", outcome.result.ParentRef)
		}
	}
	childID := NewIssueOpsID(parent.Repo, "123-child-same")
	if _, err := ReadIssueOps(stateRoot, childID); err != nil {
		t.Fatalf("expected one child record: %v", err)
	}
	parentAfter, err := ReadIssueOps(stateRoot, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if countChildRefs(parentAfter.ChildCycles, childID) != 1 {
		t.Fatalf("same-branch concurrency should leave one parent ref, got %#v", parentAfter.ChildCycles)
	}
	ref, ok := childRefByID(parentAfter.ChildCycles, childID)
	if !ok || ref.Branch != "123-child-same" || !requestedTitles[ref.Title] {
		t.Fatalf("same-branch parent ref should match one originating request, got %#v", ref)
	}
}

func TestStartIssueOpsChildConcurrentSiblings(t *testing.T) {
	stateRoot := t.TempDir()
	parent := createDelegationReadyParentForTest(t, stateRoot)
	const workers = 5
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	expectedTitles := make(map[string]string, workers)
	for i := 0; i < workers; i++ {
		expectedTitles[fmt.Sprintf("12%d-child-sibling", i)] = fmt.Sprintf("sibling %d", i)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			branch := fmt.Sprintf("12%d-child-sibling", i)
			title := fmt.Sprintf("sibling %d", i)
			result, err := startIssueOpsChildForTest(stateRoot, parent, issueops.IssueOpsChildStartRequest{
				ParentID:           parent.ID,
				Branch:             branch,
				Title:              title,
				TaskScope:          "sibling concurrency",
				AcceptanceCriteria: []string{"every sibling persists"},
			})
			if err != nil {
				errs <- err
				return
			}
			if result.ParentRef.Branch != branch || result.ParentRef.Title != title || result.ParentRef.CycleID != result.Child.ID {
				errs <- fmt.Errorf("ref mismatch for %s: %#v", branch, result.ParentRef)
				return
			}
			errs <- nil
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	parentAfter, err := ReadIssueOps(stateRoot, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(parentAfter.ChildCycles) != workers {
		t.Fatalf("expected %d sibling refs, got %#v", workers, parentAfter.ChildCycles)
	}
	for i := 0; i < workers; i++ {
		branch := fmt.Sprintf("12%d-child-sibling", i)
		childID := NewIssueOpsID(parent.Repo, branch)
		ref, ok := childRefByID(parentAfter.ChildCycles, childID)
		if !ok {
			t.Fatalf("missing child ref for %s (%s): %#v", branch, childID, parentAfter.ChildCycles)
		}
		if ref.Branch != branch || ref.Title != expectedTitles[branch] {
			t.Fatalf("child ref for %s should preserve branch/title, got %#v", branch, ref)
		}
	}
}

func createDelegationReadyParentForTest(t *testing.T, stateRoot string) issueops.IssueOpsRecord {
	t.Helper()
	repo := initIssueOpsRepo(t)
	branch := "123-parent"
	worktree := makeIssueOpsWorktreeDirForTest(t, repo, branch)
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsIntentForTest(t, stateRoot, record.ID)
	record, err = LinkIssueOpsIssue(stateRoot, record.ID, "https://github.com/example/repo/issues/123")
	if err != nil {
		t.Fatal(err)
	}
	record, err = PrepareIssueOpsBranch(stateRoot, record.ID, issueops.IssueOpsBranchPrepareRequest{Provider: "github", IssueURL: record.IssueURL, Branch: branch, BaseBranch: "main", LinkVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	record, err = LinkIssueOpsWorktree(stateRoot, record.ID, worktree)
	if err != nil {
		t.Fatal(err)
	}
	recordIssueOpsApprovedDesignForTest(t, stateRoot, record.ID)
	writeIssueOpsFile(t, worktree, "plans/parent.md", "parent plan\n")
	record, err = LinkIssueOpsPlan(stateRoot, record.ID, filepath.Join(worktree, "plans/parent.md"))
	if err != nil {
		t.Fatal(err)
	}
	record = recordIssueOpsPreparedExecutionForTest(t, stateRoot, record.ID, worktree)
	return record
}

func writeIssueOpsRecordForDelegationTest(t *testing.T, stateRoot string, record issueops.IssueOpsRecord) issueops.IssueOpsRecord {
	t.Helper()
	written, err := writeIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return written
}

func hasChildRef(refs []issueops.IssueOpsChildCycleRef, childID string) bool {
	return countChildRefs(refs, childID) > 0
}

func childRefByID(refs []issueops.IssueOpsChildCycleRef, childID string) (issueops.IssueOpsChildCycleRef, bool) {
	for _, ref := range refs {
		if ref.CycleID == childID {
			return ref, true
		}
	}
	return issueops.IssueOpsChildCycleRef{}, false
}

func countChildRefs(refs []issueops.IssueOpsChildCycleRef, childID string) int {
	count := 0
	for _, ref := range refs {
		if ref.CycleID == childID {
			count++
		}
	}
	return count
}
