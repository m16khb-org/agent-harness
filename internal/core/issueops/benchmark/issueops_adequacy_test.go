package benchmark

import (
	"sort"
	"strings"
	"testing"
)

// A4 — scorer check-adequacy (per-dimension mutation suite).
//
// The live benchmark gate (issueops benchmark run) only ever scores
// synthesized-PASSING artifacts (FromFixture builds correct artifacts from the
// fixture's expected fields), so it reports avg=min=100 by construction. That
// proves the SYNTHESIZER works, NOT that the SCORER catches defects: a
// dimension whose check silently became always-ok (a "dead check") would keep
// the gate green and escape every existing test for ~7 of the 19 dimensions
// whose only signal is a plain artifact field (intent_understanding,
// plan_quality, tdd_quality, implementation_readiness, phase_control_quality,
// branch_worktree_gate_quality, worktree_cleanup_quality).
//
// This suite closes that gap: for every dimension it mutates a fully-passing
// artifact so the dimension's discriminating signal is removed, then proves the
// dimension's OWN score row drops to a live 0. A dead always-ok check leaves its
// row at 100 and fails the assertion regardless of any co-broken sibling,
// because the contract reads the per-dimension row (dimScore), never the
// aggregate min/avg (which a co-broken sibling could mask).
//
// Design notes (S2 review):
//   - Global isolation ("mutate D, all other 18 stay 100") is structurally
//     impossible: several dimensions share an artifact field. So instead each
//     row declares the OTHER dimensions that necessarily co-drop (coupled) and
//     the suite asserts the set of dropped dimensions equals exactly
//     {D} ∪ coupled — bounding the blast radius so an over-mutation (e.g.
//     blanking the whole artifact) is caught.
//   - The FULL fixture carries NO critical-failure rules, so each mutation
//     exercises ONLY the deterministic dimension channel; the suite asserts zero
//     criticals fire. Critical-failure detection is covered separately by the
//     TestScoreIssueOpsBenchmarkArtifact*CriticalFailures tests.
//   - pioneer_skill_contribution / skill_routing_fidelity are metadata-
//     conditional: the mutation blanks only the ARTIFACT signal
//     (PioneerSkillEvidence / RoutingTrace) and keeps the fixture metadata, so
//     the dimension stays applicable and a genuine deterministic 0 (not an
//     excluded N/A) is what proves the check fired. The !NotApplicable assertion
//     rejects an N/A masquerading as a 0.
//   - The completeness guard binds the table to issueOpsBenchmarkDimensions in
//     both directions, so a future dimension cannot ship without an adequacy
//     mutator (the A5 dimension-count-regression lesson).

func adequacyFixtureForTest() IssueOpsBenchmarkFixture {
	return IssueOpsBenchmarkFixture{
		ID:                 "adequacy-full",
		Title:              "Adequacy full fixture",
		UserPrompt:         "exercise every scoring dimension",
		PioneerSkillTarget: "codd",
		ExpectedRouting:    []SkillRouting{{Phase: "plan", Skill: "codd"}},
		// No CriticalFailures on purpose: this suite tests the deterministic
		// dimension channel only.
	}
}

func adequacyArtifactForTest() IssueOpsBenchmarkArtifact {
	a := completeBenchmarkArtifactForTest()
	a.PioneerSkillEvidence = coddKeywordEvidence
	a.RoutingTrace = []SkillRouting{{Phase: "plan", Skill: "codd"}}
	return a
}

func adequacyRow(score IssueOpsBenchmarkScore, dimension string) IssueOpsDimensionScore {
	for _, d := range score.DimensionScores {
		if d.Dimension == dimension {
			return d
		}
	}
	// Sentinel: a missing dimension fails both the live-100 and live-0 checks.
	return IssueOpsDimensionScore{Dimension: dimension, Score: -1}
}

func adequacyDroppedDims(score IssueOpsBenchmarkScore) []string {
	var dropped []string
	for _, dim := range issueOpsBenchmarkDimensions {
		row := adequacyRow(score, dim)
		if row.Score == 0 && !row.NotApplicable {
			dropped = append(dropped, dim)
		}
	}
	sort.Strings(dropped)
	return dropped
}

func TestScoreIssueOpsBenchmarkArtifactEveryDimensionDiscriminates(t *testing.T) {
	fixture := adequacyFixtureForTest()

	// Baseline: a fully-passing artifact must score every dimension live at 100
	// with zero failures, so each mutation below starts from a known-good state
	// and a no-op mutator (one that fails to remove the signal) is caught by the
	// mutated-row==0 assertion rather than passing vacuously.
	baseScore := ScoreIssueOpsBenchmarkArtifact(fixture, adequacyArtifactForTest())
	if !baseScore.Passed || len(baseScore.DeterministicFailures) != 0 || len(baseScore.CriticalFailures) != 0 {
		t.Fatalf("adequacy baseline must pass cleanly: %+v", baseScore)
	}
	for _, dim := range issueOpsBenchmarkDimensions {
		row := adequacyRow(baseScore, dim)
		if row.NotApplicable || row.Score != issueOpsBenchmarkMaxScore {
			t.Fatalf("baseline dimension %q must be live at 100, got %+v", dim, row)
		}
	}

	type adequacyCase struct {
		// mutate removes ONLY the discriminating signal of the keyed dimension
		// (plus, for shared-field dimensions, whatever the coupled list declares).
		mutate func(IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact
		// coupled lists the OTHER dimensions that necessarily co-drop because they
		// read the same artifact field; empty means the mutation is single-axis.
		coupled []string
	}

	cases := map[string]adequacyCase{
		"intent_understanding": {
			// ok = ProblemSummary!="" || IssueDraft!="". Both must be blanked;
			// blanking IssueDraft co-drops issue_quality (shared field).
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.ProblemSummary = ""
				a.IssueDraft = ""
				return a
			},
			coupled: []string{"issue_quality"},
		},
		"issue_quality": {
			// Break the issue-specific label-decision clause only; ProblemSummary
			// and IssueDraft stay non-empty so intent_understanding stays 100.
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.IssueDraft = strings.ReplaceAll(a.IssueDraft, "선택 라벨: enhancement(score 0.90), 거절 라벨: documentation(score 0.20), threshold 0.70, 수동 override 없음.\n", "")
				return a
			},
		},
		"domain_contract_quality": {
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.DomainContractEvidence = ""
				return a
			},
		},
		"plan_quality": {
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.Plan = "Implement the change."
				return a
			},
		},
		"api_doc_gate_quality": {
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.APIDocGateEvidence = ""
				return a
			},
		},
		"live_evidence_quality": {
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.LiveEvidenceMatrix = ""
				return a
			},
		},
		"task_decomposition": {
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.TaskBreakdown = ""
				return a
			},
		},
		"tdd_quality": {
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.TDDPlan = ""
				return a
			},
		},
		"subagent_orchestration": {
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.SubagentPrompts = ""
				return a
			},
		},
		"review_feedback_accountability": {
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.ReviewFeedbackEvidence = ""
				return a
			},
		},
		"implementation_readiness": {
			// ok = BranchName!="" && WorktreePath!="". Blanking BranchName co-drops
			// branch_worktree_gate_quality (feature/ prefix); they share the field
			// and implementation_readiness has no private signal to isolate.
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.BranchName = ""
				return a
			},
			coupled: []string{"branch_worktree_gate_quality"},
		},
		"pr_mr_quality": {
			// Remove the issue-link clause from PRDraft only; GuidelineRef stays so
			// issue_quality's shared guideline clause is unaffected.
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.PRDraft = strings.ReplaceAll(a.PRDraft, "Issue: https://example.com/acme/agent-harness/issues/1\n", "")
				return a
			},
		},
		"phase_control_quality": {
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.PhaseChoices = ""
				return a
			},
		},
		"branch_worktree_gate_quality": {
			// Keep BranchName non-empty (implementation_readiness stays 100) but
			// drop the feature/ prefix so only the gate dimension fails.
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.BranchName = "main"
				return a
			},
		},
		"isolation_compliance": {
			// Implementation outside the worktree; WorktreePath/BranchName stay so
			// implementation_readiness and the gate dimension stay 100.
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.ImplementationLocation = "/elsewhere/outside-worktree"
				return a
			},
		},
		"completion_hygiene_quality": {
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.CompletionHygiene = ""
				return a
			},
		},
		"worktree_cleanup_quality": {
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.WorktreeCleanup = ""
				return a
			},
		},
		"pioneer_skill_contribution": {
			// Metadata stays (fixture keeps PioneerSkillTarget) so the dimension
			// remains applicable; only the artifact signal is removed.
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.PioneerSkillEvidence = ""
				return a
			},
		},
		"skill_routing_fidelity": {
			mutate: func(a IssueOpsBenchmarkArtifact) IssueOpsBenchmarkArtifact {
				a.RoutingTrace = nil
				return a
			},
		},
	}

	// Completeness guard (both directions): every scored dimension must own an
	// adequacy mutator, and the table may not carry an unknown dimension. A new
	// dimension cannot ship without proving it discriminates.
	if len(cases) != len(issueOpsBenchmarkDimensions) {
		t.Fatalf("adequacy table has %d cases but there are %d dimensions", len(cases), len(issueOpsBenchmarkDimensions))
	}
	for _, dim := range issueOpsBenchmarkDimensions {
		if _, ok := cases[dim]; !ok {
			t.Fatalf("dimension %q has no adequacy mutator (completeness guard)", dim)
		}
	}
	for dim := range cases {
		if !containsString(issueOpsBenchmarkDimensions, dim) {
			t.Fatalf("adequacy case %q is not a known dimension (completeness guard)", dim)
		}
	}

	for dim, tc := range cases {
		mutated := ScoreIssueOpsBenchmarkArtifact(fixture, tc.mutate(adequacyArtifactForTest()))

		// (1) The targeted dimension's OWN row dropped to a live deterministic 0.
		// Reading the row (not min/avg) is what makes a dead always-ok check
		// detectable even when a coupled sibling also dropped.
		row := adequacyRow(mutated, dim)
		if row.NotApplicable || row.Score != 0 {
			t.Fatalf("mutating %q must drop its own row to a live 0, got %+v", dim, row)
		}

		// (2) Bounded blast radius: exactly {dim} ∪ coupled dropped. An
		// over-mutation (blanking unrelated signal) would drop more and fail here.
		want := []string{dim}
		want = append(want, tc.coupled...)
		sort.Strings(want)
		got := adequacyDroppedDims(mutated)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("mutating %q dropped %v, want exactly %v (over/under-mutation)", dim, got, want)
		}

		// (3) The deterministic mutation must not leak into the critical channel.
		if len(mutated.CriticalFailures) != 0 {
			t.Fatalf("dimension mutation %q must not trip criticals, got %v", dim, mutated.CriticalFailures)
		}
	}
}
