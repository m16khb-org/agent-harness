package trace

import (
	"bufio"
	"encoding/json"
	"fmt"
	tracecontract "issueops/internal/contract/trace"
	"strings"

	"issueops/internal/domain/policy"
	"issueops/internal/domain/traceclassification"
)

func analyzeTraceJSONL(text string) ([]tracecontract.TraceAnalysisFinding, []string) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	events := []docUpkeepEvent{}
	failedSteps := map[string]int{}
	traceTypes := []string{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			continue
		}
		if event := docUpkeepEventFromMap(doc); event.Kind != "" || event.Summary != "" {
			events = append(events, event)
			traceTypes = append(traceTypes, "doc_upkeep_jsonl")
			continue
		}
		if stringField(doc, "event") == "step_end" && !boolField(doc, "ok") {
			step := stringField(doc, "step")
			if step == "" {
				step = "unknown step"
			}
			failedSteps[step]++
			traceTypes = append(traceTypes, "self_verify_progress_jsonl")
		}
	}
	findings := []tracecontract.TraceAnalysisFinding{}
	findings = append(findings, docUpkeepFindings(events)...)
	for _, step := range traceSortedIntKeys(failedSteps) {
		findings = append(findings, tracecontract.TraceAnalysisFinding{
			FailureClass:        "self_verify_progress_failure",
			RecurringPattern:    fmt.Sprintf("%s failed %d time(s)", policy.RedactFreeform(step), failedSteps[step]),
			ProposedKnob:        classification.ProposedKnobForStep(step),
			OverfitRisk:         "medium: progress JSONL may capture one run; rerun before changing harness behavior",
			VerificationCommand: classification.DefaultVerificationCommand(step),
		})
	}
	return findings, traceTypes
}
