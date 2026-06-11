package benchmark

import "strings"

// issueOpsPioneerSkillEvidenceComplete reports whether the artifact's
// PioneerSkillEvidence carries the distinctive-method signature of the
// fixture's targeted pioneer skill.
//
// This is a NECESSARY KEYWORD PROXY, not a proof that the skill ran or that
// its method changed the artifact: an artifact containing the required terms
// in a meaningless order still passes (covered by an explicit boundary test).
// Each signature requires every clause below (AND), with synonym sets per
// clause to reduce false negatives from legitimate rephrasing.
//
// Fixtures without a pioneer_skill_target are handled as N/A by the scorer
// and never reach this check.
func issueOpsPioneerSkillEvidenceComplete(fixture IssueOpsBenchmarkFixture, artifact IssueOpsBenchmarkArtifact) bool {
	evidence := artifact.PioneerSkillEvidence
	switch strings.ToLower(strings.TrimSpace(fixture.PioneerSkillTarget)) {
	case "dijkstra":
		// complexity-class AND scaling-evidence AND before/after comparison.
		return containsAnyFold(evidence, "complexity", "asymptotic", "big-o", "quadratic", "linearithmic") &&
			containsAnyFold(evidence, "scaling", "n=", "n →", "n ->", "benchmark", "measured at") &&
			containsAnyFold(evidence, "before", "after", "baseline")
	case "codd":
		// index decision AND write-penalty cost AND design rationale.
		return containsAnyFold(evidence, "index") &&
			containsAnyFold(evidence, "write penalty", "write cost", "insert cost") &&
			containsAnyFold(evidence, "1nf", "2nf", "3nf", "bcnf", "normal", "selectivity", "row count", "read:write")
	case "hopper":
		// reproduction AND cause isolation AND verification.
		return containsAnyFold(evidence, "reproduce") &&
			containsAnyFold(evidence, "root cause", "isolate", "hypothesis") &&
			containsAnyFold(evidence, "verify", "verified", "fix", "regression")
	case "shannon":
		// SNR AND before AND after AND at least one secondary metric.
		return containsAnyFold(evidence, "snr") &&
			containsAnyFold(evidence, "before", "baseline") &&
			containsAnyFold(evidence, "after", "improved") &&
			containsAnyFold(evidence, "entropy", "redundancy", "overhead")
	default:
		return false
	}
}
