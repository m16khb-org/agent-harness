package benchmark

import issueopscontract "issueops/internal/contract/issueops"

import "strings"

// issueOpsPioneerSkillEvidenceComplete reports whether the artifact's
// PioneerSkillEvidence carries the distinctive-method signature of the
// fixture's targeted pioneer skill.
//
// This is a deterministic artifact-structure proxy, not proof that the skill
// ran. Each signature requires method-specific labeled clauses (AND) so hollow
// keyword soup does not pass as method evidence.
//
// Fixtures without a pioneer_skill_target are handled as N/A by the scorer
// and never reach this check.
func issueOpsPioneerSkillEvidenceComplete(fixture issueopscontract.IssueOpsBenchmarkFixture, artifact issueopscontract.IssueOpsBenchmarkArtifact) bool {
	evidence := artifact.PioneerSkillEvidence
	switch strings.ToLower(strings.TrimSpace(fixture.PioneerSkillTarget)) {
	case "implementation-planning":
		return hasStructuredClause(evidence, "repo grounding", "grounding") &&
			hasStructuredClause(evidence, "decision-complete", "decision complete") &&
			hasStructuredClause(evidence, "assumptions", "defaults") &&
			hasStructuredClause(evidence, "unresolved question", "open question", "deferred risk") &&
			hasStructuredClause(evidence, "acceptance criteria", "success criteria")
	case "verified-execution":
		return hasStructuredClause(evidence, "success criteria", "criteria") &&
			hasStructuredClause(evidence, "evidence artifact", "artifact") &&
			hasStructuredClause(evidence, "cleanup receipt", "cleanup") &&
			hasStructuredClause(evidence, "verification mode", "proportionate") &&
			hasStructuredClause(evidence, "skipped checks", "skipped check")
	case "web-research":
		return hasStructuredClause(evidence, "source fan-out", "fan-out") &&
			hasStructuredClause(evidence, "source index", "sources") &&
			hasStructuredClause(evidence, "claim verification", "verification status") &&
			hasStructuredClause(evidence, "access boundary", "auth", "paywall", "protected", "inaccessible") &&
			containsAnyFold(evidence, "retrieval timestamp", "retrieved", "retrieval date", "2026-")
	case "algorithm-optimization":
		return hasStructuredClause(evidence, "hot path", "profile") &&
			hasStructuredClause(evidence, "complexity", "asymptotic", "big-o", "o(") &&
			hasStructuredClause(evidence, "scaling", "n=", "measured at") &&
			hasStructuredClause(evidence, "invariant", "correctness") &&
			hasStructuredClause(evidence, "before/after", "before", "benchmark delta", "measurement") &&
			containsAnyFold(evidence, "after", "improved")
	case "database-design":
		return hasStructuredClause(evidence, "schema", "row count", "ddl", "cardinality") &&
			hasStructuredClause(evidence, "explain", "query plan") &&
			hasStructuredClause(evidence, "index") &&
			containsAnyFold(evidence, "write penalty", "write cost", "insert cost") &&
			hasStructuredClause(evidence, "normalization", "normalisation", "anomaly", "bcnf", "3nf")
	case "issueops-debugging":
		return hasStructuredClause(evidence, "reproduction", "reproduced") &&
			hasStructuredClause(evidence, "failure signature", "signature") &&
			hasStructuredClause(evidence, "root cause", "hypothesis") &&
			hasStructuredClause(evidence, "isolation", "isolated") &&
			hasStructuredClause(evidence, "minimal fix", "fix scope", "fix boundary") &&
			hasStructuredClause(evidence, "verification", "regression proof")
	case "code-quality-metrics":
		return hasStructuredClause(evidence, "diff inventory", "scope inventory") &&
			containsAnyFold(evidence, "untracked") &&
			hasStructuredClause(evidence, "snr") &&
			containsAnyFold(evidence, "before", "baseline") &&
			containsAnyFold(evidence, "after", "improved") &&
			hasStructuredClause(evidence, "secondary metric", "second metric") &&
			containsAnyFold(evidence, "entropy", "redundancy", "overhead") &&
			hasStructuredClause(evidence, "heuristic caveat", "approximation caveat") &&
			hasStructuredClause(evidence, "no-input guard", "zero-input guard")
	case "prompt-engineering":
		return hasStructuredClause(evidence, "input/output contract", "input contract", "output contract") &&
			hasStructuredClause(evidence, "test suite") &&
			hasStructuredClause(evidence, "adversarial") &&
			hasStructuredClause(evidence, "one-variable", "one variable") &&
			hasStructuredClause(evidence, "privacy", "hidden chain-of-thought", "tool truth")
	case "git-operations":
		return hasStructuredClause(evidence, "git state proof", "state proof") &&
			hasStructuredClause(evidence, "recovery path", "backup ref") &&
			hasStructuredClause(evidence, "destructive confirmation", "confirmation gate") &&
			hasStructuredClause(evidence, "atomic scope", "atomic") &&
			hasStructuredClause(evidence, "force-with-lease", "--force-with-lease")
	case "requirements-analysis":
		return hasStructuredClause(evidence, "document scope") &&
			hasStructuredClause(evidence, "ocr evidence", "screenshot evidence") &&
			hasStructuredClause(evidence, "requirement ledger") &&
			hasStructuredClause(evidence, "contradiction") &&
			hasStructuredClause(evidence, "risk-driven recommendation")
	case "design-review":
		return hasStructuredClause(evidence, "essential complexity") &&
			hasStructuredClause(evidence, "accidental complexity") &&
			hasStructuredClause(evidence, "second-system effect") &&
			hasStructuredClause(evidence, "conceptual integrity") &&
			hasStructuredClause(evidence, "go/no-go verdict", "no-go verdict")
	case "meeting-notes":
		return hasStructuredClause(evidence, "source fidelity") &&
			hasStructuredClause(evidence, "decision log") &&
			hasStructuredClause(evidence, "action owners") &&
			hasStructuredClause(evidence, "uncertainty") &&
			hasStructuredClause(evidence, "canvas handoff")
	case "issueops":
		return hasStructuredClause(evidence, "durable state record", "state record") &&
			hasStructuredClause(evidence, "phase routing", "routing") &&
			hasStructuredClause(evidence, "flow evidence", "issue", "plan", "tdd") &&
			hasStructuredClause(evidence, "hook boundary", "hook") &&
			hasStructuredClause(evidence, "cleanup/readiness", "readiness evidence", "cleanup")
	default:
		return false
	}
}

func hasStructuredClause(text string, markers ...string) bool {
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, ":") {
			continue
		}
		if containsAnyFold(line, markers...) {
			return true
		}
	}
	return false
}
