package benchmark

import issueopscontract "agent-harness/internal/contract/issueops"

import "strings"

// issueOpsSkillRoutingFidelityComplete reports whether the artifact's recorded
// RoutingTrace contains every skill-at-phase pairing the fixture expects (A5).
//
// This is a RECORDED-TRACE PROXY, not proof of live in-CI skill routing. The
// deterministic benchmark run is tautological for this dimension because
// FromFixture synthesizes RoutingTrace from the fixture's own ExpectedRouting
// (parallel to the pioneer keyword proxy's "not live-routing proof" caveat at
// issueops_pioneer_checks.go and issueops_benchmark_score.go). Real
// discrimination comes from (a) the tampered-trace boundary test and (b) future
// REAL traces recorded during non-CI issueops runs.
//
// Each expected (phase, skill) must match a SINGLE trace entry on BOTH fields
// (case-insensitive, trimmed) — not two independent any-scans — so a trace
// where the right skill fired at the WRONG phase fails.
//
// Fixtures without ExpectedRouting are handled as N/A by the scorer and never
// reach this check.
func issueOpsSkillRoutingFidelityComplete(fixture issueopscontract.IssueOpsBenchmarkFixture, artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	return RoutingFidelity(fixture.ExpectedRouting, artifact.RoutingTrace).OK
}

// RoutingFidelity is the shared core of skill_routing_fidelity. It reports
// whether observed covers every expected (phase, skill) pairing and which are
// missing. Reused for both the benchmark dimension (observed = artifact trace)
// and live scoring of a real run (observed = the recorded RoutingTrace), so a
// real run is scored by the same logic instead of a synthesized tautology.
func RoutingFidelity(expected, observed []issueopscontract.SkillRouting) RoutingFidelityResult {
	var missing []issueopscontract.SkillRouting
	for _, e := range expected {
		if !routingTraceHasPairing(observed, e) {
			missing = append(missing, e)
		}
	}
	return RoutingFidelityResult{OK: len(missing) == 0, Missing: missing}
}

func routingTraceHasPairing(trace []issueopscontract.SkillRouting, expected issueopscontract.SkillRouting) bool {
	for _, entry := range trace {
		if strings.EqualFold(strings.TrimSpace(entry.Phase), strings.TrimSpace(expected.Phase)) &&
			strings.EqualFold(strings.TrimSpace(entry.Skill), strings.TrimSpace(expected.Skill)) {
			return true
		}
	}
	return false
}
