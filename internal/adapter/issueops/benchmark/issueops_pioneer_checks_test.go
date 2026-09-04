package benchmark

import issueopscontract "issueops/internal/contract/issueops"

import "testing"

func pioneerCheckForTest(target, evidence string) bool {
	return issueOpsPioneerSkillEvidenceComplete(
		issueopscontract.IssueOpsBenchmarkFixture{ID: "pioneer-" + target, PioneerSkillTarget: target},
		issueopscontract.IssueOpsBenchmarkArtifact{PioneerSkillEvidence: evidence},
	)
}

func TestPioneerSkillSignatures(t *testing.T) {
	cases := []struct {
		name     string
		target   string
		evidence string
		want     bool
	}{
		// algorithm-optimization: hot-path proof AND complexity-class AND scaling-test AND
		// correctness invariant AND before/after measurement.
		{"algorithm-optimization full", "algorithm-optimization",
			"Hot path: pprof shows matcher at 87% CPU\nComplexity: O(n^2) -> O(n log n)\nScaling evidence: N=100/1000/10000 table\nCorrectness invariant: sorted candidate set preserves all matches\nBefore/after measurement: baseline 4.1s, after 0.2s", true},
		{"algorithm-optimization synonyms", "algorithm-optimization",
			"Profile: CPU hot path confirmed in matcher\nBig-O claim: quadratic to linearithmic\nScaling table: measured at n=100, n=1000, n=10000\nInvariant: same order IDs emitted once\nBenchmark delta: before 4.1s after 0.2s", true},
		{"algorithm-optimization missing hot path clause", "algorithm-optimization",
			"Complexity: O(n^2) -> O(n log n)\nScaling evidence: N=100/1000/10000 table\nCorrectness invariant: sorted candidate set preserves all matches\nBefore/after measurement: baseline 4.1s, after 0.2s", false},
		{"algorithm-optimization missing complexity clause", "algorithm-optimization",
			"Hot path: pprof shows matcher at 87% CPU\nScaling evidence: N=100/1000/10000 table\nCorrectness invariant: sorted candidate set preserves all matches\nBefore/after measurement: baseline 4.1s, after 0.2s", false},
		{"algorithm-optimization missing scaling clause", "algorithm-optimization",
			"Hot path: pprof shows matcher at 87% CPU\nComplexity: O(n^2) -> O(n log n)\nCorrectness invariant: sorted candidate set preserves all matches\nBefore/after measurement: baseline 4.1s, after 0.2s", false},
		{"algorithm-optimization missing invariant clause", "algorithm-optimization",
			"Hot path: pprof shows matcher at 87% CPU\nComplexity: O(n^2) -> O(n log n)\nScaling evidence: N=100/1000/10000 table\nBefore/after measurement: baseline 4.1s, after 0.2s", false},
		{"algorithm-optimization missing before/after clause", "algorithm-optimization",
			"Hot path: pprof shows matcher at 87% CPU\nComplexity: O(n^2) -> O(n log n)\nScaling evidence: N=100/1000/10000 table\nCorrectness invariant: sorted candidate set preserves all matches", false},
		// database-design: schema/row-count AND query plan AND index/write-penalty AND
		// normalization/anomaly rationale.
		{"database-design full", "database-design",
			"Schema/row count: orders has 12M rows\nEXPLAIN evidence: seq scan before, index scan after\nIndex tradeoff: covering index, write penalty +8% insert cost\nNormalization rationale: 3NF retained; no update anomaly", true},
		{"database-design synonyms", "database-design",
			"DDL and cardinality: orders row count 12M\nQuery plan: EXPLAIN ANALYZE before/after compared\nIndex recommendation: partial index; write cost grows 8%\nAnomaly rationale: BCNF held; read:write ratio justifies no denormalization", true},
		{"database-design missing index clause", "database-design",
			"Schema/row count: orders has 12M rows\nEXPLAIN evidence: seq scan before, index scan after\nNormalization rationale: 3NF retained; no update anomaly", false},
		{"database-design missing write-penalty clause", "database-design",
			"Schema/row count: orders has 12M rows\nEXPLAIN evidence: seq scan before, index scan after\nIndex recommendation: covering index on user_id_created_at\nNormalization rationale: 3NF retained; no update anomaly", false},
		{"database-design missing rationale clause", "database-design",
			"Schema/row count: orders has 12M rows\nEXPLAIN evidence: seq scan before, index scan after\nIndex tradeoff: covering index, write penalty +8% insert cost", false},
		// debugging: reproduction/failure signature AND cause hypothesis AND
		// isolation AND minimal fix boundary AND verification.
		{"debugging full", "issueops-debugging",
			"Reproduction: go test -run X exits 1\nFailure signature: expected 200 got 401\nRoot cause hypothesis: stale cache key\nIsolation: bisect narrowed to auth/cache.go\nMinimal fix boundary: one cache key builder\nVerification: regression test and rerun passed", true},
		{"debugging synonyms", "issueops-debugging",
			"Reproduced: command fails deterministically\nSignature: missing config file path in stderr\nHypothesis: config lookup uses cwd\nIsolated cause: trace diff points to loader\nFix scope: path resolver only\nRegression proof: test added and verified", true},
		{"debugging missing reproduce clause", "issueops-debugging",
			"Failure signature: expected 200 got 401\nRoot cause hypothesis: stale cache key\nIsolation: bisect narrowed to auth/cache.go\nMinimal fix boundary: one cache key builder\nVerification: regression test and rerun passed", false},
		{"debugging missing cause clause", "issueops-debugging",
			"Reproduction: go test -run X exits 1\nFailure signature: expected 200 got 401\nIsolation: bisect narrowed to auth/cache.go\nMinimal fix boundary: one cache key builder\nVerification: regression test and rerun passed", false},
		{"debugging missing verify clause", "issueops-debugging",
			"Reproduction: go test -run X exits 1\nFailure signature: expected 200 got 401\nRoot cause hypothesis: stale cache key\nIsolation: bisect narrowed to auth/cache.go\nMinimal fix boundary: one cache key builder", false},
		// code-quality-metrics: scoped diff inventory AND SNR before/after AND secondary
		// metric AND heuristic caveat AND no-input guard.
		{"code-quality-metrics full", "code-quality-metrics",
			"Diff inventory: staged, unstaged, and untracked files listed\nSNR before/after: 0.62 -> 0.81\nSecondary metric: entropy down 12%\nHeuristic caveat: shell metric approximate\nNo-input guard: total=0 reports insufficient-input", true},
		{"code-quality-metrics synonyms", "code-quality-metrics",
			"Scope inventory: git status covers tracked and untracked files\nSignal/noise baseline: snr before 0.62, improved after 0.81\nSecond metric: redundancy removed in 3 blocks\nApproximation caveat: AST check recommended\nZero-input guard: no measurable diff returns no-op", true},
		{"code-quality-metrics missing snr clause", "code-quality-metrics",
			"Diff inventory: staged, unstaged, and untracked files listed\nSecondary metric: entropy down 12%\nHeuristic caveat: shell metric approximate\nNo-input guard: total=0 reports insufficient-input", false},
		{"code-quality-metrics missing before clause", "code-quality-metrics",
			"Diff inventory: staged, unstaged, and untracked files listed\nSNR after: 0.81\nSecondary metric: entropy down 12%\nHeuristic caveat: shell metric approximate\nNo-input guard: total=0 reports insufficient-input", false},
		{"code-quality-metrics missing after clause", "code-quality-metrics",
			"Diff inventory: staged, unstaged, and untracked files listed\nSNR before: 0.62\nSecondary metric: entropy down 12%\nHeuristic caveat: shell metric approximate\nNo-input guard: total=0 reports insufficient-input", false},
		{"code-quality-metrics missing second-metric clause", "code-quality-metrics",
			"Diff inventory: staged, unstaged, and untracked files listed\nSNR before/after: 0.62 -> 0.81\nHeuristic caveat: shell metric approximate\nNo-input guard: total=0 reports insufficient-input", false},
		// Additional pioneer targets: each has a distinctive method signature.
		{"implementation-planning full", "implementation-planning",
			"Repo grounding: read AGENTS.md and benchmark code\nDecision-complete plan: tasks have owners and dependencies\nAssumptions/defaults: default to local fixtures\nUnresolved questions: none blocking; deferred risk recorded\nAcceptance criteria: validation commands and artifacts listed", true},
		{"verified-execution full", "verified-execution",
			"Success criteria: every requirement mapped to pass/fail\nEvidence artifact: command stdout captured at evidence/G1-C1.txt\nCleanup receipt: temp dir removed and verified\nVerification mode: proportionate CLI check for docs-only change\nSkipped checks: browser QA skipped as not applicable with reason", true},
		{"web-research full", "web-research",
			"Source fan-out: official docs, changelog, package index\nSource index: URLs cited with retrieval timestamp 2026-06-23T10:00:00Z\nClaim verification: confirmed, single-sourced, disputed table\nAccess boundary: paywalled source marked inaccessible without bypass", true},
		{"prompt-engineering full", "prompt-engineering",
			"Input/output contract: prompt receives issue text and returns JSON\nTest suite: 3 happy cases and 2 edge cases\nAdversarial cases: hidden-reasoning and fake-tool injection\nOne-variable iteration: only moved format spec in v2\nPrivacy/tool truth: no hidden chain-of-thought; tools verified or illustrative", true},
		{"git-operations full", "git-operations",
			"Git state proof: status, branch, log, and worktree list captured\nRecovery path: backup ref refs/heads/backup/main-pre-reset verified\nDestructive confirmation gate: exact reset command requires user approval\nAtomic scope: one intent per commit\nForce-with-lease rule: never use raw force push", true},
		{"issueops full", "issueops",
			"Durable state record: issueops state id and readiness gates recorded\nPhase routing: problem -> issue -> plan -> implement -> feedback -> pr -> cleanup\nFlow evidence: issue, plan, TDD, subagent decision, feedback, PR all linked\nHook boundary: hooks do not create issues, edit files, or run tests\nCleanup/readiness evidence: strict readiness and cleanup choices recorded", true},
		{"implementation-planning missing acceptance criteria", "implementation-planning",
			"Repo grounding: read AGENTS.md\nDecision-complete plan: tasks have owners\nAssumptions/defaults: default to local fixtures\nUnresolved questions: none blocking", false},
		{"verified-execution missing cleanup receipt", "verified-execution",
			"Success criteria: every requirement mapped to pass/fail\nEvidence artifact: command stdout captured\nVerification mode: proportionate CLI check\nSkipped checks: browser QA skipped as not applicable with reason", false},
		{"web-research missing retrieval timestamp", "web-research",
			"Source fan-out: official docs, changelog, package index\nSource index: URLs cited\nClaim verification: confirmed table\nAccess boundary: paywalled source inaccessible", false},
		{"prompt-engineering missing concrete cases", "prompt-engineering",
			"Input/output contract: prompt receives issue text and returns JSON\nAdversarial cases: hidden-reasoning and fake-tool injection\nOne-variable iteration: only moved format spec in v2\nPrivacy/tool truth: no hidden chain-of-thought; tools verified or illustrative", false},
		{"git-operations missing recovery path", "git-operations",
			"Git state proof: status, branch, log, and worktree list captured\nDestructive confirmation gate: exact reset command requires user approval\nAtomic scope: one intent per commit\nForce-with-lease rule: never use raw force push", false},
		{"issueops missing hook boundary", "issueops",
			"Durable state record: issueops state id and readiness gates recorded\nPhase routing: problem -> issue -> plan -> implement -> feedback -> pr -> cleanup\nFlow evidence: issue, plan, TDD, subagent decision, feedback, PR all linked\nCleanup/readiness evidence: strict readiness and cleanup choices recorded", false},
		// Boundary: hollow keyword soup is rejected. The deterministic check is
		// still a proxy, but it now requires method-specific artifact structure.
		{"algorithm-optimization hollow keyword soup rejected", "algorithm-optimization",
			"complexity scaling before after invariant hot path O(n) words in no meaningful order", false},
		{"database-design hollow keyword soup rejected", "database-design",
			"index write penalty row count explain normal form recommendation", false},
		{"debugging hollow keyword soup rejected", "issueops-debugging",
			"reproduce root cause verify regression hypothesis signature isolate fix", false},
		{"prompt-engineering hollow keyword soup rejected", "prompt-engineering",
			"test suite adversarial iteration input output contract hidden reasoning tool truth", false},
		// Unknown / empty targets are never satisfied.
		{"unknown target", "knuth", "complexity scaling before after", false},
		{"empty evidence", "algorithm-optimization", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pioneerCheckForTest(tc.target, tc.evidence); got != tc.want {
				t.Fatalf("target=%s evidence=%q: got %v want %v", tc.target, tc.evidence, got, tc.want)
			}
		})
	}
}
