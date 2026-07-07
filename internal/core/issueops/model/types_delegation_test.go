package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestIssueOpsRecordDelegationRoundTrip(t *testing.T) {
	rec := IssueOpsRecord{
		OK:            true,
		SchemaVersion: IssueOpsCurrentSchemaVersion,
		ID:            "io-parent",
		Repo:          "/repo/example",
		Branch:        "123-parent",
		Phase:         IssueOpsPhaseImplement,
		Delegation: &IssueOpsDelegationContract{
			ParentCycleID:      "io-root",
			TaskScope:          "implement delegated model types",
			AcceptanceCriteria: []string{"delegation contract round-trips", "child refs round-trip"},
			ParentPlanPath:     "docs/superpowers/plans/orchestration.md",
			ChildIssueURL:      "https://github.com/example/repo/issues/124",
			DelegatedAt:        "2026-07-07T00:00:00Z",
		},
		ChildCycles: []IssueOpsChildCycleRef{
			{
				CycleID:            "io-child-a",
				Branch:             "124-child-a",
				Title:              "child A",
				ChildIssueURL:      "https://github.com/example/repo/issues/124",
				CreatedAt:          "2026-07-07T00:01:00Z",
				ValidationVerdict:  "accepted",
				ValidationEvidence: []string{"go test ./internal/core/issueops/model"},
				ValidatedAt:        "2026-07-07T00:02:00Z",
			},
			{
				CycleID:       "io-child-b",
				Branch:        "125-child-b",
				Title:         "child B",
				ChildIssueURL: "https://github.com/example/repo/issues/125",
				CreatedAt:     "2026-07-07T00:03:00Z",
			},
		},
		CreatedAt: "2026-07-07T00:00:00Z",
		UpdatedAt: "2026-07-07T00:03:00Z",
	}

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	var got IssueOpsRecord
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal record: %v", err)
	}
	if got.Delegation == nil {
		t.Fatalf("delegation did not round-trip: %#v", got)
	}
	if got.Delegation.ParentCycleID != rec.Delegation.ParentCycleID || got.Delegation.TaskScope != rec.Delegation.TaskScope {
		t.Fatalf("unexpected delegation contract: %#v", got.Delegation)
	}
	if len(got.Delegation.AcceptanceCriteria) != 2 || got.Delegation.AcceptanceCriteria[1] != "child refs round-trip" {
		t.Fatalf("acceptance criteria did not round-trip: %#v", got.Delegation.AcceptanceCriteria)
	}
	if len(got.ChildCycles) != 2 {
		t.Fatalf("child cycles did not round-trip: %#v", got.ChildCycles)
	}
	if got.ChildCycles[0].ValidationVerdict != "accepted" || got.ChildCycles[0].ValidationEvidence[0] != "go test ./internal/core/issueops/model" {
		t.Fatalf("validation verdict/evidence did not round-trip: %#v", got.ChildCycles[0])
	}
	if got.ChildCycles[1].CycleID != "io-child-b" || got.ChildCycles[1].Branch != "125-child-b" {
		t.Fatalf("second child ref did not round-trip: %#v", got.ChildCycles[1])
	}
}

func TestIssueOpsRecordWithoutDelegationOmitsKeys(t *testing.T) {
	rec := IssueOpsRecord{ID: "io-plain", Repo: "/repo/example", Phase: IssueOpsPhaseProblem}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	s := string(data)
	for _, key := range []string{"delegation", "child_cycles"} {
		if strings.Contains(s, key) {
			t.Fatalf("record without delegation should omit %q, got %s", key, s)
		}
	}
}
