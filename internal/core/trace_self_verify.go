package core

import (
	"fmt"
	"strings"
)

func selfVerifySummaryFindings(doc map[string]any) []TraceAnalysisFinding {
	summary := nestedMap(doc, "summary")
	if summary == nil {
		summary = doc
	}
	failedSteps := intField(summary, "failed_steps")
	failedStep := stringField(summary, "failed_step")
	failureClass := stringField(summary, "failure_class")
	if failedSteps == 0 && failedStep == "" && failureClass == "" {
		return nil
	}
	if failureClass == "" {
		failureClass = "self_verification_failure"
	}
	pattern := failedStep
	if pattern == "" {
		pattern = "self-verification reported failed steps"
	}
	if clusters, ok := summary["failure_clusters"].([]any); ok && len(clusters) > 0 {
		parts := []string{}
		for _, item := range clusters {
			cluster, ok := item.(map[string]any)
			if !ok {
				continue
			}
			step := stringField(cluster, "step")
			count := intField(cluster, "count")
			if step != "" {
				parts = append(parts, fmt.Sprintf("%s failed %d time(s)", redactFreeform(step), count))
			}
		}
		if len(parts) > 0 {
			pattern = strings.Join(parts, "; ")
		}
	}
	return []TraceAnalysisFinding{{
		FailureClass:        redactFreeform(failureClass),
		RecurringPattern:    redactFreeform(pattern),
		ProposedKnob:        proposedKnobForStep(failedStep),
		OverfitRisk:         overfitRiskForClass(failureClass),
		VerificationCommand: firstString(summary, "rerun_commands", defaultTraceVerificationCommand(failedStep)),
	}}
}

func traceTypesForJSON(doc map[string]any) []string {
	types := []string{}
	if nestedMap(doc, "summary") != nil || stringField(doc, "failure_class") != "" || intField(doc, "failed_steps") > 0 {
		types = append(types, "self_verify_summary")
	}
	if nestedMap(doc, "guard") != nil {
		types = append(types, "guard_result")
	}
	event := docUpkeepEventFromMap(doc)
	if event.Kind != "" || event.Summary != "" {
		types = append(types, "doc_upkeep_json")
	}
	return types
}
