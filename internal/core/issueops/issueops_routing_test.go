package issueops

import "testing"

func TestRecordIssueOpsRouting(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "1-routing"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := RecordIssueOpsRouting(stateRoot, record.ID, "", "codd"); err == nil {
		t.Fatal("empty phase must be rejected")
	}
	if _, err := RecordIssueOpsRouting(stateRoot, record.ID, "plan", ""); err == nil {
		t.Fatal("empty skill must be rejected")
	}

	if _, err := RecordIssueOpsRouting(stateRoot, record.ID, "plan", "codd"); err != nil {
		t.Fatal(err)
	}
	rec, err := RecordIssueOpsRouting(stateRoot, record.ID, "implement", "dijkstra")
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.RoutingTrace) != 2 {
		t.Fatalf("expected 2 routing entries, got %d", len(rec.RoutingTrace))
	}

	// Idempotent: re-recording the same (phase, skill) must not duplicate.
	rec, err = RecordIssueOpsRouting(stateRoot, record.ID, "plan", "codd")
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.RoutingTrace) != 2 {
		t.Fatalf("idempotent record should keep 2 entries, got %d", len(rec.RoutingTrace))
	}

	sr := RoutingTraceAsSkillRouting(rec)
	if len(sr) != 2 || sr[0].Phase != "plan" || sr[0].Skill != "codd" {
		t.Fatalf("unexpected SkillRouting projection: %#v", sr)
	}
}

func TestScoreLiveRoutingFidelity(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "2-score"})
	if err != nil {
		t.Fatal(err)
	}
	record, err = RecordIssueOpsRouting(stateRoot, record.ID, "plan", "codd")
	if err != nil {
		t.Fatal(err)
	}

	// Live trace covers the expectation → pass.
	if r := ScoreLiveRoutingFidelity(record, []SkillRouting{{Phase: "plan", Skill: "codd"}}); !r.OK || len(r.Missing) != 0 {
		t.Fatalf("covered routing should pass: %#v", r)
	}

	// A pairing the run never routed is flagged — real discrimination, unlike the
	// benchmark's synthesized (tautological) trace.
	r := ScoreLiveRoutingFidelity(record, []SkillRouting{{Phase: "plan", Skill: "codd"}, {Phase: "implement", Skill: "dijkstra"}})
	if r.OK || len(r.Missing) != 1 || r.Missing[0].Skill != "dijkstra" {
		t.Fatalf("missing pairing must be flagged: %#v", r)
	}
}
