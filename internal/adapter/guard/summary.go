package guard

import (
	guardcontract "agent-harness/internal/contract/guard"
	"fmt"
)

func dedupeGuardFindings(findings []guardcontract.GuardFinding) []guardcontract.GuardFinding {
	seen := map[string]bool{}
	out := []guardcontract.GuardFinding{}
	for _, finding := range findings {
		key := finding.Severity + "\x00" + finding.Rule + "\x00" + finding.File + "\x00" + fmt.Sprint(finding.Line) + "\x00" + finding.Evidence
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, finding)
	}
	return out
}

func guardSeverityRank(severity string) int {
	switch severity {
	case "block":
		return 0
	case "warn":
		return 1
	case "review":
		return 2
	default:
		return 3
	}
}
