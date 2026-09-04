package issueopsnext

import (
	"context"
	"strings"
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

func testPorts(entries []issueopsinventorycontract.ListEntry, records map[string]issueopscontract.IssueOpsRecord) Ports {
	return Ports{
		ListCycles: func(ctx context.Context, stateRoot, repo string) (issueopsinventorycontract.ListResult, error) {
			return issueopsinventorycontract.ListResult{OK: true, Entries: entries}, nil
		},
		ReadRecord: func(stateRoot, id string) (issueopscontract.IssueOpsRecord, error) {
			return records[id], nil
		},
		Completion: func(record issueopscontract.IssueOpsRecord, phase issueopscontract.IssueOpsPhase) issueopscontract.IssueOpsReadiness {
			return issueopscontract.IssueOpsReadiness{OK: true, Ready: true}
		},
		SourceRoot: func(cwd string) string { return "/repo" },
		Env:        func(string) string { return "" },
		Now:        func() time.Time { return time.Unix(0, 0) },
	}
}

func TestNextWithoutCyclesOffersStart(t *testing.T) {
	service := NewService(testPorts(nil, nil))
	result, err := service.Next(context.Background(), "/state", "/repo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK || result.Stage.Key != "none" || result.CWDRole != "source" {
		t.Fatalf("an empty state must offer a fresh cycle: %+v", result)
	}
	if !strings.Contains(result.NextCommand, "issueops start --repo /repo") {
		t.Fatalf("next command must start a cycle in the source root: %q", result.NextCommand)
	}
	if result.Exits.AbandonCommand != "" {
		t.Fatalf("without a cycle there is nothing to abandon: %+v", result.Exits)
	}
}

func TestNextSelectsTheCycleOwningTheWorkingDirectory(t *testing.T) {
	entries := []issueopsinventorycontract.ListEntry{
		{ID: "io-a", Phase: issueopscontract.IssueOpsPhaseImplement, Branch: "12-a", WorkspaceRoot: "/repo.worktrees/12-a"},
		{ID: "io-b", Phase: issueopscontract.IssueOpsPhaseImplement, Branch: "13-b", WorkspaceRoot: "/repo.worktrees/13-b"},
	}
	records := map[string]issueopscontract.IssueOpsRecord{
		"io-b": {ID: "io-b", Repo: "/repo", Branch: "13-b", Phase: issueopscontract.IssueOpsPhaseImplement},
	}
	service := NewService(testPorts(entries, records))
	result, err := service.Next(context.Background(), "/state", "/repo.worktrees/13-b", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Selected == nil || result.Selected.ID != "io-b" {
		t.Fatalf("the cycle owning the cwd must win: %+v", result.Selected)
	}
	if result.CWDRole != "worktree" {
		t.Fatalf("cwd role must be worktree, got %q", result.CWDRole)
	}
}

func TestNextReportsAmbiguityInsteadOfGuessing(t *testing.T) {
	entries := []issueopsinventorycontract.ListEntry{
		{ID: "io-a", Phase: issueopscontract.IssueOpsPhaseImplement, Branch: "12-a"},
		{ID: "io-b", Phase: issueopscontract.IssueOpsPhasePlan, Branch: "13-b"},
	}
	service := NewService(testPorts(entries, nil))
	result, err := service.Next(context.Background(), "/state", "/repo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.OK || result.Stage.Key != "ambiguous" || len(result.Candidates) != 2 {
		t.Fatalf("two active cycles must be reported as candidates: %+v", result)
	}
	if result.Selected != nil {
		t.Fatalf("an ambiguous lookup must not select a cycle: %+v", result.Selected)
	}
}

// done 사이클은 자동 선택 후보가 아니다. 남은 활성 사이클이 하나면 그것을 고른다.
func TestNextIgnoresDoneCyclesWhenAutoSelecting(t *testing.T) {
	entries := []issueopsinventorycontract.ListEntry{
		{ID: "io-old", Phase: issueopscontract.IssueOpsPhaseDone, Branch: "11-old"},
		{ID: "io-live", Phase: issueopscontract.IssueOpsPhaseGrill, Branch: "12-live"},
	}
	records := map[string]issueopscontract.IssueOpsRecord{
		"io-live": {ID: "io-live", Repo: "/repo", Branch: "12-live", Phase: issueopscontract.IssueOpsPhaseGrill},
	}
	service := NewService(testPorts(entries, records))
	result, err := service.Next(context.Background(), "/state", "/repo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Selected == nil || result.Selected.ID != "io-live" {
		t.Fatalf("the only active cycle must be selected: %+v", result.Selected)
	}
	if result.Stage.Key != "issue" {
		t.Fatalf("a grill cycle without an issue is stage 1, got %q", result.Stage.Key)
	}
}

// local readiness는 git을 읽으므로 5~8단계 phase에서만 호출해야 한다.
func TestNextReadsLocalReadinessOnlyAfterImplement(t *testing.T) {
	calls := 0
	ports := testPorts(
		[]issueopsinventorycontract.ListEntry{{ID: "io-a", Phase: issueopscontract.IssueOpsPhasePlan, Branch: "12-a"}},
		map[string]issueopscontract.IssueOpsRecord{
			"io-a": {ID: "io-a", Repo: "/repo", Branch: "12-a", Phase: issueopscontract.IssueOpsPhasePlan},
		},
	)
	ports.LocalReadiness = func(record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness {
		calls++
		return issueopscontract.IssueOpsReadiness{OK: true, Ready: true}
	}
	if _, err := NewService(ports).Next(context.Background(), "/state", "/repo", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 0 {
		t.Fatalf("a plan-phase cycle must not read local readiness, got %d calls", calls)
	}
}

func TestNextRejectsMissingDependencies(t *testing.T) {
	if _, err := NewService(Ports{}).Next(context.Background(), "/state", "/repo", ""); err == nil {
		t.Fatal("an unconfigured service must fail closed")
	}
}
