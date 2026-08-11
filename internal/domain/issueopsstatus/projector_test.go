package issueopsstatus

import (
	"reflect"
	"slices"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
)

func TestProjectorDerivesDeterministicLedger(t *testing.T) {
	projector := NewProjector(func(
		_ issueopscontract.IssueOpsRecord,
		phase issueopscontract.IssueOpsPhase,
	) issueopscontract.IssueOpsReadiness {
		if phase == issueopscontract.IssueOpsPhaseProblem {
			return issueopscontract.IssueOpsReadiness{Ready: true}
		}
		return issueopscontract.IssueOpsReadiness{Missing: []string{"missing_" + string(phase)}}
	})
	record := issueopscontract.IssueOpsRecord{Phase: issueopscontract.IssueOpsPhaseGrill}

	left := projector.Derive(record)
	right := projector.Derive(record)
	if !reflect.DeepEqual(left, right) {
		t.Fatalf("derived ledger is not deterministic:\nleft=%#v\nright=%#v", left, right)
	}
	if problem := left[issueopscontract.IssueOpsPhaseProblem]; len(problem.Artifacts) == 0 || len(problem.Missing) != 0 {
		t.Fatalf("ready phase projection is incomplete: %#v", problem)
	}
	if grill := left[issueopscontract.IssueOpsPhaseGrill]; len(grill.Missing) == 0 {
		t.Fatalf("incomplete phase must expose missing keys: %#v", grill)
	}
}

func TestProjectorBackfillsWithoutReplacingPersistedEntries(t *testing.T) {
	projector := NewProjector(func(
		issueopscontract.IssueOpsRecord,
		issueopscontract.IssueOpsPhase,
	) issueopscontract.IssueOpsReadiness {
		return issueopscontract.IssueOpsReadiness{Ready: true}
	})
	record := issueopscontract.IssueOpsRecord{
		Phase: issueopscontract.IssueOpsPhasePlan,
		PhaseLedger: issueopscontract.IssueOpsPhaseLedger{
			issueopscontract.IssueOpsPhaseProblem: {
				Phase:       issueopscontract.IssueOpsPhaseProblem,
				CompletedAt: "2026-06-29T00:01:00Z",
			},
		},
	}

	projected := projector.Project(record)
	if len(projected.PhaseLedger) != len(issueopscontract.IssueOpsPhases) {
		t.Fatalf("partial ledger was not backfilled: %#v", projected.PhaseLedger)
	}
	if got := projected.PhaseLedger[issueopscontract.IssueOpsPhaseProblem].CompletedAt; got != "2026-06-29T00:01:00Z" {
		t.Fatalf("persisted entry was replaced: %q", got)
	}
}

func TestArtifactKeysIndexChildrenOnlyAtPR(t *testing.T) {
	if !slices.Contains(ArtifactKeys(issueopscontract.IssueOpsPhasePR), "children_complete") {
		t.Fatal("PR artifact keys must include children_complete")
	}
	if slices.Contains(ArtifactKeys(issueopscontract.IssueOpsPhaseImplement), "children_complete") {
		t.Fatal("implement artifact keys must not include children_complete")
	}
}
