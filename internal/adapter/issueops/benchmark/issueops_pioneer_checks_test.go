package benchmark

import "testing"

func pioneerCheckForTest(target, evidence string) bool {
	return issueOpsPioneerSkillEvidenceComplete(
		IssueOpsBenchmarkFixture{ID: "pioneer-" + target, PioneerSkillTarget: target},
		IssueOpsBenchmarkArtifact{PioneerSkillEvidence: evidence},
	)
}

func TestPioneerSkillSignatures(t *testing.T) {
	cases := []struct {
		name     string
		target   string
		evidence string
		want     bool
	}{
		// dijkstra: hot-path proof AND complexity-class AND scaling-test AND
		// correctness invariant AND before/after measurement.
		{"dijkstra full", "dijkstra",
			"Hot path: pprof shows matcher at 87% CPU\nComplexity: O(n^2) -> O(n log n)\nScaling evidence: N=100/1000/10000 table\nCorrectness invariant: sorted candidate set preserves all matches\nBefore/after measurement: baseline 4.1s, after 0.2s", true},
		{"dijkstra synonyms", "dijkstra",
			"Profile: CPU hot path confirmed in matcher\nBig-O claim: quadratic to linearithmic\nScaling table: measured at n=100, n=1000, n=10000\nInvariant: same order IDs emitted once\nBenchmark delta: before 4.1s after 0.2s", true},
		{"dijkstra missing hot path clause", "dijkstra",
			"Complexity: O(n^2) -> O(n log n)\nScaling evidence: N=100/1000/10000 table\nCorrectness invariant: sorted candidate set preserves all matches\nBefore/after measurement: baseline 4.1s, after 0.2s", false},
		{"dijkstra missing complexity clause", "dijkstra",
			"Hot path: pprof shows matcher at 87% CPU\nScaling evidence: N=100/1000/10000 table\nCorrectness invariant: sorted candidate set preserves all matches\nBefore/after measurement: baseline 4.1s, after 0.2s", false},
		{"dijkstra missing scaling clause", "dijkstra",
			"Hot path: pprof shows matcher at 87% CPU\nComplexity: O(n^2) -> O(n log n)\nCorrectness invariant: sorted candidate set preserves all matches\nBefore/after measurement: baseline 4.1s, after 0.2s", false},
		{"dijkstra missing invariant clause", "dijkstra",
			"Hot path: pprof shows matcher at 87% CPU\nComplexity: O(n^2) -> O(n log n)\nScaling evidence: N=100/1000/10000 table\nBefore/after measurement: baseline 4.1s, after 0.2s", false},
		{"dijkstra missing before/after clause", "dijkstra",
			"Hot path: pprof shows matcher at 87% CPU\nComplexity: O(n^2) -> O(n log n)\nScaling evidence: N=100/1000/10000 table\nCorrectness invariant: sorted candidate set preserves all matches", false},
		// codd: schema/row-count AND query plan AND index/write-penalty AND
		// normalization/anomaly rationale.
		{"codd full", "codd",
			"Schema/row count: orders has 12M rows\nEXPLAIN evidence: seq scan before, index scan after\nIndex tradeoff: covering index, write penalty +8% insert cost\nNormalization rationale: 3NF retained; no update anomaly", true},
		{"codd synonyms", "codd",
			"DDL and cardinality: orders row count 12M\nQuery plan: EXPLAIN ANALYZE before/after compared\nIndex recommendation: partial index; write cost grows 8%\nAnomaly rationale: BCNF held; read:write ratio justifies no denormalization", true},
		{"codd missing index clause", "codd",
			"Schema/row count: orders has 12M rows\nEXPLAIN evidence: seq scan before, index scan after\nNormalization rationale: 3NF retained; no update anomaly", false},
		{"codd missing write-penalty clause", "codd",
			"Schema/row count: orders has 12M rows\nEXPLAIN evidence: seq scan before, index scan after\nIndex recommendation: covering index on user_id_created_at\nNormalization rationale: 3NF retained; no update anomaly", false},
		{"codd missing rationale clause", "codd",
			"Schema/row count: orders has 12M rows\nEXPLAIN evidence: seq scan before, index scan after\nIndex tradeoff: covering index, write penalty +8% insert cost", false},
		// hopper: reproduction/failure signature AND cause hypothesis AND
		// isolation AND minimal fix boundary AND verification.
		{"hopper full", "hopper",
			"Reproduction: go test -run X exits 1\nFailure signature: expected 200 got 401\nRoot cause hypothesis: stale cache key\nIsolation: bisect narrowed to auth/cache.go\nMinimal fix boundary: one cache key builder\nVerification: regression test and rerun passed", true},
		{"hopper synonyms", "hopper",
			"Reproduced: command fails deterministically\nSignature: missing config file path in stderr\nHypothesis: config lookup uses cwd\nIsolated cause: trace diff points to loader\nFix scope: path resolver only\nRegression proof: test added and verified", true},
		{"hopper missing reproduce clause", "hopper",
			"Failure signature: expected 200 got 401\nRoot cause hypothesis: stale cache key\nIsolation: bisect narrowed to auth/cache.go\nMinimal fix boundary: one cache key builder\nVerification: regression test and rerun passed", false},
		{"hopper missing cause clause", "hopper",
			"Reproduction: go test -run X exits 1\nFailure signature: expected 200 got 401\nIsolation: bisect narrowed to auth/cache.go\nMinimal fix boundary: one cache key builder\nVerification: regression test and rerun passed", false},
		{"hopper missing verify clause", "hopper",
			"Reproduction: go test -run X exits 1\nFailure signature: expected 200 got 401\nRoot cause hypothesis: stale cache key\nIsolation: bisect narrowed to auth/cache.go\nMinimal fix boundary: one cache key builder", false},
		// shannon: scoped diff inventory AND SNR before/after AND secondary
		// metric AND heuristic caveat AND no-input guard.
		{"shannon full", "shannon",
			"Diff inventory: staged, unstaged, and untracked files listed\nSNR before/after: 0.62 -> 0.81\nSecondary metric: entropy down 12%\nHeuristic caveat: shell metric approximate\nNo-input guard: total=0 reports insufficient-input", true},
		{"shannon synonyms", "shannon",
			"Scope inventory: git status covers tracked and untracked files\nSignal/noise baseline: snr before 0.62, improved after 0.81\nSecond metric: redundancy removed in 3 blocks\nApproximation caveat: AST check recommended\nZero-input guard: no measurable diff returns no-op", true},
		{"shannon missing snr clause", "shannon",
			"Diff inventory: staged, unstaged, and untracked files listed\nSecondary metric: entropy down 12%\nHeuristic caveat: shell metric approximate\nNo-input guard: total=0 reports insufficient-input", false},
		{"shannon missing before clause", "shannon",
			"Diff inventory: staged, unstaged, and untracked files listed\nSNR after: 0.81\nSecondary metric: entropy down 12%\nHeuristic caveat: shell metric approximate\nNo-input guard: total=0 reports insufficient-input", false},
		{"shannon missing after clause", "shannon",
			"Diff inventory: staged, unstaged, and untracked files listed\nSNR before: 0.62\nSecondary metric: entropy down 12%\nHeuristic caveat: shell metric approximate\nNo-input guard: total=0 reports insufficient-input", false},
		{"shannon missing second-metric clause", "shannon",
			"Diff inventory: staged, unstaged, and untracked files listed\nSNR before/after: 0.62 -> 0.81\nHeuristic caveat: shell metric approximate\nNo-input guard: total=0 reports insufficient-input", false},
		// Additional pioneer targets: each has a distinctive method signature.
		{"von-neumann full", "von-neumann",
			"Repo grounding: read AGENTS.md and benchmark code\nDecision-complete plan: tasks have owners and dependencies\nAssumptions/defaults: default to local fixtures\nUnresolved questions: none blocking; deferred risk recorded\nAcceptance criteria: validation commands and artifacts listed", true},
		{"turing full", "turing",
			"Success criteria: every requirement mapped to pass/fail\nEvidence artifact: command stdout captured at evidence/G1-C1.txt\nCleanup receipt: temp dir removed and verified\nVerification mode: proportionate CLI check for docs-only change\nSkipped checks: browser QA skipped as not applicable with reason", true},
		{"berners-lee full", "berners-lee",
			"Source fan-out: official docs, changelog, package index\nSource index: URLs cited with retrieval timestamp 2026-06-23T10:00:00Z\nClaim verification: confirmed, single-sourced, disputed table\nAccess boundary: paywalled source marked inaccessible without bypass", true},
		{"karpathy full", "karpathy",
			"Input/output contract: prompt receives issue text and returns JSON\nTest suite: 3 happy cases and 2 edge cases\nAdversarial cases: hidden-reasoning and fake-tool injection\nOne-variable iteration: only moved format spec in v2\nPrivacy/tool truth: no hidden chain-of-thought; tools verified or illustrative", true},
		{"torvalds full", "torvalds",
			"Git state proof: status, branch, log, and worktree list captured\nRecovery path: backup ref refs/heads/backup/main-pre-reset verified\nDestructive confirmation gate: exact reset command requires user approval\nAtomic scope: one intent per commit\nForce-with-lease rule: never use raw force push", true},
		{"issueops full", "issueops",
			"Durable state record: issueops state id and readiness gates recorded\nPhase routing: problem -> issue -> plan -> implement -> feedback -> pr -> cleanup\nFlow evidence: issue, plan, TDD, subagent decision, feedback, PR all linked\nHook boundary: hooks do not create issues, edit files, or run tests\nCleanup/readiness evidence: strict readiness and cleanup choices recorded", true},
		{"von-neumann missing acceptance criteria", "von-neumann",
			"Repo grounding: read AGENTS.md\nDecision-complete plan: tasks have owners\nAssumptions/defaults: default to local fixtures\nUnresolved questions: none blocking", false},
		{"turing missing cleanup receipt", "turing",
			"Success criteria: every requirement mapped to pass/fail\nEvidence artifact: command stdout captured\nVerification mode: proportionate CLI check\nSkipped checks: browser QA skipped as not applicable with reason", false},
		{"berners-lee missing retrieval timestamp", "berners-lee",
			"Source fan-out: official docs, changelog, package index\nSource index: URLs cited\nClaim verification: confirmed table\nAccess boundary: paywalled source inaccessible", false},
		{"karpathy missing concrete cases", "karpathy",
			"Input/output contract: prompt receives issue text and returns JSON\nAdversarial cases: hidden-reasoning and fake-tool injection\nOne-variable iteration: only moved format spec in v2\nPrivacy/tool truth: no hidden chain-of-thought; tools verified or illustrative", false},
		{"torvalds missing recovery path", "torvalds",
			"Git state proof: status, branch, log, and worktree list captured\nDestructive confirmation gate: exact reset command requires user approval\nAtomic scope: one intent per commit\nForce-with-lease rule: never use raw force push", false},
		{"issueops missing hook boundary", "issueops",
			"Durable state record: issueops state id and readiness gates recorded\nPhase routing: problem -> issue -> plan -> implement -> feedback -> pr -> cleanup\nFlow evidence: issue, plan, TDD, subagent decision, feedback, PR all linked\nCleanup/readiness evidence: strict readiness and cleanup choices recorded", false},
		// Boundary: hollow keyword soup is rejected. The deterministic check is
		// still a proxy, but it now requires method-specific artifact structure.
		{"dijkstra hollow keyword soup rejected", "dijkstra",
			"complexity scaling before after invariant hot path O(n) words in no meaningful order", false},
		{"codd hollow keyword soup rejected", "codd",
			"index write penalty row count explain normal form recommendation", false},
		{"hopper hollow keyword soup rejected", "hopper",
			"reproduce root cause verify regression hypothesis signature isolate fix", false},
		{"karpathy hollow keyword soup rejected", "karpathy",
			"test suite adversarial iteration input output contract hidden reasoning tool truth", false},
		// Unknown / empty targets are never satisfied.
		{"unknown target", "knuth", "complexity scaling before after", false},
		{"empty evidence", "dijkstra", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pioneerCheckForTest(tc.target, tc.evidence); got != tc.want {
				t.Fatalf("target=%s evidence=%q: got %v want %v", tc.target, tc.evidence, got, tc.want)
			}
		})
	}
}
