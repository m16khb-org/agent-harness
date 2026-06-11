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
		// dijkstra: complexity-class AND scaling-test AND before/after.
		{"dijkstra full", "dijkstra",
			"profiled hot path: complexity O(n^2) -> O(n log n); scaling test N=100->10000; before 4.1s after 0.2s", true},
		{"dijkstra synonyms", "dijkstra",
			"asymptotic cost drops from quadratic to linearithmic; measured at n=100, n=1000, n=10000; baseline 4.1s vs 0.2s", true},
		{"dijkstra missing complexity clause", "dijkstra",
			"scaling test N=100->10000; before 4.1s after 0.2s", false},
		{"dijkstra missing scaling clause", "dijkstra",
			"complexity O(n^2) -> O(n log n); before 4.1s after 0.2s improvement", false},
		{"dijkstra missing before/after clause", "dijkstra",
			"complexity O(n^2) -> O(n log n); scaling test verified", false},
		// codd: index AND write-penalty AND design-rationale.
		{"codd full", "codd",
			"covering index on (user_id, created_at); write penalty: +8% insert cost; selectivity 0.99, row count 12M", true},
		{"codd synonyms", "codd",
			"partial index chosen; insert cost grows 8%; read:write ratio 40:1 justifies it; BCNF held", true},
		{"codd missing index clause", "codd",
			"write penalty acceptable; selectivity high; normalization to 3NF", false},
		{"codd missing write-penalty clause", "codd",
			"covering index on (user_id, created_at); selectivity 0.99, row count 12M", false},
		{"codd missing rationale clause", "codd",
			"covering index added; write penalty +8%", false},
		// hopper: reproduce AND cause-isolation AND verification.
		{"hopper full", "hopper",
			"reproduced the failure with go test -run X; root cause: stale cache key; fix verified by rerun", true},
		{"hopper synonyms", "hopper",
			"reproduce steps captured; hypothesis confirmed via isolate bisect; regression test added", true},
		{"hopper missing reproduce clause", "hopper",
			"root cause: stale cache key; fix verified", false},
		{"hopper missing cause clause", "hopper",
			"reproduced the failure; fix verified by rerun", false},
		{"hopper missing verify clause", "hopper",
			"reproduced the failure; root cause: stale cache key", false},
		// shannon: SNR AND before AND after AND second-metric.
		{"shannon full", "shannon",
			"SNR before 0.62 baseline -> after 0.81; entropy down 12%", true},
		{"shannon synonyms", "shannon",
			"snr baseline 0.62, improved to 0.81; redundancy removed in 3 blocks", true},
		{"shannon missing snr clause", "shannon",
			"baseline 0.62 -> after 0.81; entropy down", false},
		{"shannon missing before clause", "shannon",
			"SNR after 0.81 improved; entropy down 12%", false},
		{"shannon missing after clause", "shannon",
			"SNR before 0.62 baseline; entropy measured", false},
		{"shannon missing second-metric clause", "shannon",
			"SNR before 0.62 baseline -> after 0.81", false},
		// Boundary: keyword soup passes BY DESIGN — this check is a necessary
		// keyword proxy, not a proof the skill's method actually ran. Kept as
		// an explicit documented limitation, mirroring the plan's honesty rule.
		{"dijkstra hollow keyword soup passes (documented proxy limit)", "dijkstra",
			"complexity scaling before after O(n) words in no meaningful order", true},
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
