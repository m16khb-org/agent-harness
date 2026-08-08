package benchmark

import (
	issueopscontract "agent-harness/internal/contract/issueops"
	"path/filepath"
	"strings"
	"testing"
)

// These A/B tests verify that the DETECTOR discriminates signature-present vs
// signature-absent evidence through the run/compare/gate wiring. They do NOT
// measure live skill routing: both artifacts are hand-authored, so this is a
// regression gate for the scoring path, not proof that issueops invoked the
// skills (the plan's honesty rule: unmeasured != failing).

func pioneerABFixturesForTest(t *testing.T) []issueopscontract.IssueOpsBenchmarkFixture {
	t.Helper()
	fixtures, err := LoadIssueOpsBenchmarkFixtures(filepath.Join("..", "..", "..", "..", "testdata", "issueops", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	var pioneers []issueopscontract.IssueOpsBenchmarkFixture
	for _, fixture := range fixtures {
		if strings.HasPrefix(fixture.ID, "pioneer-") {
			pioneers = append(pioneers, fixture)
		}
	}
	if len(pioneers) != 10 {
		t.Fatalf("expected 10 pioneer fixtures, got %d", len(pioneers))
	}
	return pioneers
}

func pioneerABEvidenceForTest(target string) string {
	switch target {
	case "von-neumann":
		return "Repo grounding: AGENTS.md and benchmark symbols inspected\nDecision-complete plan: tasks have owners and dependencies\nAssumptions/defaults: default fixture path recorded\nUnresolved questions: no blockers; deferred risks named\nAcceptance criteria: validation commands and artifacts listed"
	case "turing":
		return "Success criteria: every requirement mapped to pass/fail\nEvidence artifact: command stdout captured\nCleanup receipt: temp dir removed and verified\nVerification mode: proportionate CLI check\nSkipped checks: browser QA skipped with reason"
	case "berners-lee":
		return "Source fan-out: official docs, changelog, package index\nSource index: cited URLs with retrieval timestamp\nClaim verification: confirmed/single-sourced/disputed table\nAccess boundary: protected source inaccessible without bypass"
	case "dijkstra":
		return "Hot path: pprof shows matcher at 87% CPU\nComplexity: O(n^2) -> O(n log n)\nScaling evidence: N=100/1000/10000 table\nCorrectness invariant: sorted candidates preserve matches\nBefore/after measurement: baseline 4.1s after 0.2s"
	case "codd":
		return "Schema/row count: orders has 12M rows\nEXPLAIN evidence: seq scan before index scan after\nIndex tradeoff: covering index with write penalty +8% insert cost\nNormalization rationale: 3NF retained no update anomaly"
	case "hopper":
		return "Reproduction: go test exits 1\nFailure signature: intermittent webhook retry timeout\nRoot cause hypothesis: retry timer races\nIsolation: trace diff narrowed to scheduler\nMinimal fix boundary: retry timer only\nVerification: regression test rerun passed"
	case "shannon":
		return "Diff inventory: staged unstaged and untracked files listed\nSNR before/after: 0.62 -> 0.81\nSecondary metric: entropy and redundancy re-measured\nHeuristic caveat: shell metrics approximate\nNo-input guard: total=0 reports insufficient-input"
	case "karpathy":
		return "Input/output contract: prompt receives issue text returns JSON\nTest suite: 3 happy cases and 2 edge cases\nAdversarial cases: hidden reasoning and fake tool injection\nOne-variable iteration: only moved output spec\nPrivacy/tool truth: no hidden chain-of-thought; tools verified or illustrative"
	case "torvalds":
		return "Git state proof: status branch log and worktree list captured\nRecovery path: backup ref verified\nDestructive confirmation gate: exact reset command requires approval\nAtomic scope: one intent per commit\nForce-with-lease rule: no raw force push"
	case "issueops":
		return "Durable state record: issueops id and readiness gates recorded\nPhase routing: problem issue plan implement feedback pr cleanup\nFlow evidence: issue plan TDD subagent decision feedback PR linked\nHook boundary: hooks do not create issues edit files or run tests\nCleanup/readiness evidence: strict readiness and cleanup choices recorded"
	default:
		return ""
	}
}

func pioneerABRunForTest(t *testing.T, fixtures []issueopscontract.IssueOpsBenchmarkFixture, withEvidence bool) IssueOpsBenchmarkRunResult {
	t.Helper()
	artifacts := make(map[string]issueopscontract.IssueOpsBenchmarkArtifact, len(fixtures))
	for _, fixture := range fixtures {
		artifact := completeBenchmarkArtifactForTest()
		if withEvidence {
			artifact.PioneerSkillEvidence = pioneerABEvidenceForTest(fixture.PioneerSkillTarget)
		} else {
			artifact.PioneerSkillEvidence = ""
		}
		// Satisfy any routing expectation so this A/B isolates the pioneer
		// signal; routing fidelity has its own dedicated tests.
		artifact.RoutingTrace = fixture.ExpectedRouting
		artifacts[fixture.ID] = artifact
	}
	run, err := RunIssueOpsBenchmark(IssueOpsBenchmarkRunRequest{Fixtures: fixtures, Artifacts: artifacts})
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestPioneerSignaturePresentVsAbsentAB(t *testing.T) {
	fixtures := pioneerABFixturesForTest(t)
	absent := pioneerABRunForTest(t, fixtures, false)
	present := pioneerABRunForTest(t, fixtures, true)

	compare := CompareIssueOpsBenchmarkRuns(absent, present)
	if !compare.Improved || compare.AverageScoreDelta <= 0 {
		t.Fatalf("signature-present run must improve over absent: %+v", compare)
	}
	if len(compare.Regressions) != 0 {
		t.Fatalf("unexpected regressions: %v", compare.Regressions)
	}
	if present.CriticalFailureCount != 0 {
		t.Fatalf("present run must clear criticals, got %d", present.CriticalFailureCount)
	}
	if absent.CriticalFailureCount != 10 {
		t.Fatalf("absent run must trip the pioneer critical on all 10 fixtures, got %d", absent.CriticalFailureCount)
	}
}

// When the candidate run has no pioneer fixtures at all, the dimension is
// absent from its minimums map; the comparator must treat that as
// not-comparable rather than reading the map's 0.0 zero value and reporting a
// phantom regression (reviewer-reproduced latent gap).
func TestPioneerDimensionAbsenceIsNotARegression(t *testing.T) {
	pioneers := pioneerABFixturesForTest(t)
	baseline := pioneerABRunForTest(t, pioneers, true)

	workflowOnly := issueopscontract.IssueOpsBenchmarkFixture{ID: "workflow-only"}
	candidate, err := RunIssueOpsBenchmark(IssueOpsBenchmarkRunRequest{
		Fixtures:  []issueopscontract.IssueOpsBenchmarkFixture{workflowOnly},
		Artifacts: map[string]issueopscontract.IssueOpsBenchmarkArtifact{"workflow-only": completeBenchmarkArtifactForTest()},
	})
	if err != nil {
		t.Fatal(err)
	}

	compare := CompareIssueOpsBenchmarkRuns(baseline, candidate)
	for _, dimension := range compare.Regressions {
		if dimension == "pioneer_skill_contribution" {
			t.Fatalf("absent pioneer dimension must not be a phantom regression: %+v", compare)
		}
	}
}

func TestPioneerGateRejectsSignatureRegression(t *testing.T) {
	fixtures := pioneerABFixturesForTest(t)
	baseline := pioneerABRunForTest(t, fixtures, true)

	// Candidate drops one fixture's signature: the gate must catch the
	// pioneer dimension regression and discard the candidate.
	artifacts := make(map[string]issueopscontract.IssueOpsBenchmarkArtifact, len(fixtures))
	for i, fixture := range fixtures {
		artifact := completeBenchmarkArtifactForTest()
		artifact.PioneerSkillEvidence = pioneerABEvidenceForTest(fixture.PioneerSkillTarget)
		if i == 0 {
			artifact.PioneerSkillEvidence = ""
		}
		artifact.RoutingTrace = fixture.ExpectedRouting
		artifacts[fixture.ID] = artifact
	}
	candidateRun, err := RunIssueOpsBenchmark(IssueOpsBenchmarkRunRequest{Fixtures: fixtures, Artifacts: artifacts})
	if err != nil {
		t.Fatal(err)
	}

	result := EvaluateIssueOpsAutoresearchGate(IssueOpsAutoresearchGateRequest{
		Candidate: IssueOpsAutoresearchCandidate{
			ID:               "pioneer-ab-gate",
			Hypothesis:       "dropping a pioneer signature must be discarded",
			TargetDimensions: []string{"pioneer_skill_contribution"},
			EditSurface:      []string{"skills/**", "internal/core/issueops/**"},
		},
		BaselineRun:  baseline,
		CandidateRun: candidateRun,
		ChangedPaths: []string{"internal/core/issueops/benchmark/issueops_pioneer_checks.go"},
	})
	if result.KeepCandidate {
		t.Fatalf("gate must discard a candidate that regresses the pioneer dimension: %+v", result)
	}
	foundRegression := false
	for _, dimension := range result.TargetDimensionRegressions {
		if dimension == "pioneer_skill_contribution" {
			foundRegression = true
		}
	}
	if !foundRegression {
		t.Fatalf("expected pioneer_skill_contribution in target regressions: %+v", result)
	}
}
