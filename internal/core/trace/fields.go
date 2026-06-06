package trace

import (
	"sort"
	"strings"

	"agent-harness/internal/core/policy"
)

func nestedMap(doc map[string]any, key string) map[string]any {
	raw, ok := doc[key]
	if !ok {
		return nil
	}
	child, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return child
}

func stringField(doc map[string]any, key string) string {
	raw, ok := doc[key]
	if !ok {
		return ""
	}
	if s, ok := raw.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func intField(doc map[string]any, key string) int {
	raw, ok := doc[key]
	if !ok {
		return 0
	}
	switch v := raw.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func boolField(doc map[string]any, key string) bool {
	raw, ok := doc[key]
	if !ok {
		return true
	}
	v, ok := raw.(bool)
	return ok && v
}

func stringSliceField(doc map[string]any, key string) []string {
	raw, ok := doc[key]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := []string{}
	for _, item := range items {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func firstString(doc map[string]any, key, fallback string) string {
	items := stringSliceField(doc, key)
	if len(items) == 0 {
		return fallback
	}
	return policy.RedactFreeform(items[0])
}

func redactStringSlice(items []string) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = policy.RedactFreeform(item)
	}
	return out
}

func traceSortedIntKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func uniqSortedTraceStrings(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = true
		}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func dedupeTraceFindings(findings []TraceAnalysisFinding) []TraceAnalysisFinding {
	seen := map[string]bool{}
	out := []TraceAnalysisFinding{}
	for _, finding := range findings {
		key := finding.FailureClass + "\x00" + finding.RecurringPattern + "\x00" + finding.ProposedKnob
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, finding)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FailureClass != out[j].FailureClass {
			return out[i].FailureClass < out[j].FailureClass
		}
		return out[i].RecurringPattern < out[j].RecurringPattern
	})
	return out
}
