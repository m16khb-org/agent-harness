package core

import (
	"fmt"
	"strings"
)

func docUpkeepFindings(events []DocUpkeepEvent) []TraceAnalysisFinding {
	if len(events) == 0 {
		return nil
	}
	byTarget := map[string]int{}
	for _, event := range events {
		target := strings.Join(event.TargetDocs, ",")
		if target == "" {
			target = event.Kind
		}
		if target == "" {
			target = "doc_upkeep"
		}
		byTarget[target]++
	}
	findings := []TraceAnalysisFinding{}
	for _, target := range traceSortedIntKeys(byTarget) {
		findings = append(findings, TraceAnalysisFinding{
			FailureClass:        "lifecycle_doc_upkeep",
			RecurringPattern:    fmt.Sprintf("%s queued %d time(s)", redactFreeform(target), byTarget[target]),
			ProposedKnob:        "route pending upkeep into completion evidence instead of leaving lifecycle queue stale",
			OverfitRisk:         "low: append-only lifecycle reminders should not alter task execution",
			VerificationCommand: "go test ./internal/core -run Lifecycle -count=1",
		})
	}
	return findings
}

func docUpkeepEventFromMap(doc map[string]any) DocUpkeepEvent {
	if event := nestedMap(doc, "event"); event != nil {
		doc = event
	}
	return DocUpkeepEvent{
		Kind:       stringField(doc, "kind"),
		TargetDocs: stringSliceField(doc, "target_docs"),
		Summary:    redactFreeform(stringField(doc, "summary")),
		Evidence:   redactStringSlice(stringSliceField(doc, "evidence")),
		Source:     stringField(doc, "source"),
		Status:     stringField(doc, "status"),
	}
}
