package benchmark

import (
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/domain/pioneerskill"
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
	expected := len(pioneerskill.Names()) + 1
	if len(pioneers) != expected {
		t.Fatalf("expected %d targeted fixtures, got %d", expected, len(pioneers))
	}
	return pioneers
}

func pioneerABEvidenceForTest(target string) string {
	switch target {
	case "implementation-planning":
		return "Repo grounding: AGENTS.md and benchmark symbols inspected\nDecision-complete plan: tasks have owners and dependencies\nAssumptions/defaults: default fixture path recorded\nUnresolved questions: no blockers; deferred risks named\nAcceptance criteria: validation commands and artifacts listed"
	case "verified-execution":
		return "Success criteria: every requirement mapped to pass/fail\nEvidence artifact: command stdout captured\nCleanup receipt: temp dir removed and verified\nVerification mode: proportionate CLI check\nSkipped checks: browser QA skipped with reason"
	case "web-research":
		return "Source fan-out: official docs, changelog, package index\nSource index: cited URLs with retrieval timestamp\nClaim verification: confirmed/single-sourced/disputed table\nAccess boundary: protected source inaccessible without bypass"
	case "algorithm-optimization":
		return "Hot path: pprof shows matcher at 87% CPU\nComplexity: O(n^2) -> O(n log n)\nScaling evidence: N=100/1000/10000 table\nCorrectness invariant: sorted candidates preserve matches\nBefore/after measurement: baseline 4.1s after 0.2s"
	case "database-design":
		return "Schema/row count: orders has 12M rows\nEXPLAIN evidence: seq scan before index scan after\nIndex tradeoff: covering index with write penalty +8% insert cost\nNormalization rationale: 3NF retained no update anomaly"
	case "debugging":
		return "Reproduction: go test exits 1\nFailure signature: intermittent webhook retry timeout\nRoot cause hypothesis: retry timer races\nIsolation: trace diff narrowed to scheduler\nMinimal fix boundary: retry timer only\nVerification: regression test rerun passed"
	case "code-quality-metrics":
		return "Diff inventory: staged unstaged and untracked files listed\nSNR before/after: 0.62 -> 0.81\nSecondary metric: entropy and redundancy re-measured\nHeuristic caveat: shell metrics approximate\nNo-input guard: total=0 reports insufficient-input"
	case "prompt-engineering":
		return "Input/output contract: prompt receives issue text returns JSON\nTest suite: 3 happy cases and 2 edge cases\nAdversarial cases: hidden reasoning and fake tool injection\nOne-variable iteration: only moved output spec\nPrivacy/tool truth: no hidden chain-of-thought; tools verified or illustrative"
	case "git-operations":
		return "Git state proof: status branch log and worktree list captured\nRecovery path: backup ref verified\nDestructive confirmation gate: exact reset command requires approval\nAtomic scope: one intent per commit\nForce-with-lease rule: no raw force push"
	case "requirements-analysis":
		return "Document scope: Korean planning document and embedded visuals\nOCR evidence: small text marked uncertain\nRequirement ledger: body table and implementation requirements mapped\nContradiction: email-only table conflicts with social-login body\nRisk-driven recommendation: confirm conflict before implementation"
	case "design-review":
		return "Essential complexity: one bounded CLI behavior change\nAccidental complexity: workflow engine queue and policy DSL removed\nSecond-system effect: broad platform rewrite rejected\nConceptual integrity: one existing command path retained\nGO/NO-GO verdict: NO-GO broad plan; GO narrowed change"
	case "meeting-notes":
		return "Source fidelity: synthetic transcript preserved without invented facts\nDecision log: production deployment remains undecided\nAction owners: backend owner checks the error dashboard\nUncertainty: deployment date is explicitly unknown\nCanvas handoff: minutes and tracking fields prepared"
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
	if absent.CriticalFailureCount != len(fixtures) {
		t.Fatalf("absent run must trip the pioneer critical on all %d fixtures, got %d", len(fixtures), absent.CriticalFailureCount)
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
