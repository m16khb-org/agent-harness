package trace

import (
	"fmt"
	"sort"

	"agent-harness/internal/domain/policy"
)

func guardFindings(doc map[string]any) []TraceAnalysisFinding {
	guard := nestedMap(doc, "guard")
	if guard == nil {
		guard = doc
	}
	rawFindings, ok := guard["findings"].([]any)
	if !ok || len(rawFindings) == 0 {
		return nil
	}
	byRule := map[string]int{}
	for _, raw := range rawFindings {
		finding, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		rule := stringField(finding, "rule")
		if rule == "" {
			rule = "guard_finding"
		}
		byRule[rule]++
	}
	rules := make([]string, 0, len(byRule))
	for rule := range byRule {
		rules = append(rules, rule)
	}
	sort.Strings(rules)
	out := []TraceAnalysisFinding{}
	for _, rule := range rules {
		out = append(out, TraceAnalysisFinding{
			FailureClass:        "guard_" + policy.RedactFreeform(rule),
			RecurringPattern:    fmt.Sprintf("%s reported %d time(s)", policy.RedactFreeform(rule), byRule[rule]),
			ProposedKnob:        "adjust guard rule documentation or source pattern only if repeated false positives are confirmed",
			OverfitRisk:         "medium: guard changes can overfit to one file; verify with fixture coverage",
			VerificationCommand: "go test ./internal/core -run Guard -count=1",
		})
	}
	return out
}
