package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

const TraceAnalysisKind = "trace_analysis"

type TraceAnalyzeRequest struct {
	Input string `json:"input"`
}

type TraceAnalyzeResult struct {
	OK           bool                   `json:"ok"`
	Kind         string                 `json:"kind"`
	Input        string                 `json:"input"`
	InputSource  string                 `json:"input_source"`
	TraceTypes   []string               `json:"trace_types"`
	FindingCount int                    `json:"finding_count"`
	Findings     []TraceAnalysisFinding `json:"findings"`
	Warnings     []string               `json:"warnings,omitempty"`
}

type TraceAnalysisFinding struct {
	FailureClass        string `json:"failure_class"`
	RecurringPattern    string `json:"recurring_pattern"`
	ProposedKnob        string `json:"proposed_knob"`
	OverfitRisk         string `json:"overfit_risk"`
	VerificationCommand string `json:"verification_command"`
}

type traceAnalysisInput struct {
	Source string
	Body   []byte
}

func TraceAnalyze(req TraceAnalyzeRequest) (TraceAnalyzeResult, error) {
	input := strings.TrimSpace(req.Input)
	result := TraceAnalyzeResult{
		OK:         false,
		Kind:       TraceAnalysisKind,
		Input:      input,
		TraceTypes: []string{},
		Findings:   []TraceAnalysisFinding{},
		Warnings:   []string{},
	}
	if input == "" {
		return result, fmt.Errorf("trace analyze input is required")
	}
	loaded, err := loadTraceAnalysisInput(input)
	if err != nil {
		return result, err
	}
	result.InputSource = loaded.Source
	if len(strings.TrimSpace(string(loaded.Body))) == 0 {
		return result, fmt.Errorf("trace analyze input is empty")
	}
	findings, traceTypes, warnings := analyzeTraceBytes(loaded.Body)
	result.TraceTypes = traceTypes
	result.Findings = findings
	result.FindingCount = len(findings)
	result.Warnings = warnings
	result.OK = true
	return result, nil
}

func loadTraceAnalysisInput(input string) (traceAnalysisInput, error) {
	if input == "-" {
		b, err := os.ReadFile("/dev/stdin")
		return traceAnalysisInput{Source: "stdin", Body: b}, err
	}
	if b, err := os.ReadFile(input); err == nil {
		return traceAnalysisInput{Source: "file", Body: b}, nil
	}
	state, err := StateRead(input)
	if err != nil {
		return traceAnalysisInput{}, fmt.Errorf("read trace input as file or state key %q: %w", input, err)
	}
	return traceAnalysisInput{Source: "state", Body: []byte(state.Record.Content)}, nil
}

func analyzeTraceBytes(body []byte) ([]TraceAnalysisFinding, []string, []string) {
	text := strings.TrimSpace(string(body))
	findings := []TraceAnalysisFinding{}
	traceTypes := []string{}
	warnings := []string{}

	if strings.HasPrefix(text, "{") {
		var doc map[string]any
		if err := json.Unmarshal([]byte(text), &doc); err != nil {
			if strings.Contains(text, "\n") {
				jsonlFindings, jsonlTypes := analyzeTraceJSONL(text)
				if len(jsonlFindings) > 0 {
					return dedupeTraceFindings(jsonlFindings), uniqSortedTraceStrings(jsonlTypes), warnings
				}
			}
			return findings, traceTypes, []string{"invalid_json:" + err.Error()}
		}
		findings = append(findings, selfVerifySummaryFindings(doc)...)
		findings = append(findings, guardFindings(doc)...)
		if event := docUpkeepEventFromMap(doc); event.Kind != "" || event.Summary != "" {
			findings = append(findings, docUpkeepFindings([]DocUpkeepEvent{event})...)
		}
		if len(findings) == 0 {
			warnings = append(warnings, "no_supported_trace_findings")
		} else {
			traceTypes = append(traceTypes, traceTypesForJSON(doc)...)
		}
		return dedupeTraceFindings(findings), uniqSortedTraceStrings(traceTypes), warnings
	}

	jsonlFindings, jsonlTypes := analyzeTraceJSONL(text)
	findings = append(findings, jsonlFindings...)
	traceTypes = append(traceTypes, jsonlTypes...)
	if len(findings) == 0 {
		warnings = append(warnings, "no_supported_trace_findings")
	}
	return dedupeTraceFindings(findings), uniqSortedTraceStrings(traceTypes), warnings
}

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
			FailureClass:        "guard_" + redactFreeform(rule),
			RecurringPattern:    fmt.Sprintf("%s reported %d time(s)", redactFreeform(rule), byRule[rule]),
			ProposedKnob:        "adjust guard rule documentation or source pattern only if repeated false positives are confirmed",
			OverfitRisk:         "medium: guard changes can overfit to one file; verify with fixture coverage",
			VerificationCommand: "go test ./internal/core -run Guard -count=1",
		})
	}
	return out
}

func analyzeTraceJSONL(text string) ([]TraceAnalysisFinding, []string) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	events := []DocUpkeepEvent{}
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
		if stringField(doc, "event") == "step_end" && boolField(doc, "ok") == false {
			step := stringField(doc, "step")
			if step == "" {
				step = "unknown step"
			}
			failedSteps[step]++
			traceTypes = append(traceTypes, "self_verify_progress_jsonl")
		}
	}
	findings := []TraceAnalysisFinding{}
	findings = append(findings, docUpkeepFindings(events)...)
	for _, step := range traceSortedIntKeys(failedSteps) {
		findings = append(findings, TraceAnalysisFinding{
			FailureClass:        "self_verify_progress_failure",
			RecurringPattern:    fmt.Sprintf("%s failed %d time(s)", redactFreeform(step), failedSteps[step]),
			ProposedKnob:        proposedKnobForStep(step),
			OverfitRisk:         "medium: progress JSONL may capture one run; rerun before changing harness behavior",
			VerificationCommand: defaultTraceVerificationCommand(step),
		})
	}
	return findings, traceTypes
}

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

func proposedKnobForStep(step string) string {
	step = strings.ToLower(step)
	switch {
	case strings.Contains(step, "contract") || strings.Contains(step, "golden"):
		return "tighten CLI/MCP contract golden coverage and update schema intentionally"
	case strings.Contains(step, "redaction") || strings.Contains(step, "secret"):
		return "extend redaction audit fixtures before adding any new logging surface"
	case strings.Contains(step, "daemon"):
		return "add daemon stale-lock/socket resilience fixture before changing runtime behavior"
	case strings.Contains(step, "policy") || strings.Contains(step, "guard"):
		return "add deterministic policy or guard fixture for the repeated failure"
	case strings.Contains(step, "go test") || strings.Contains(step, "test"):
		return "reduce failing test to a fixture-backed regression before broad harness changes"
	case strings.Contains(step, "build"):
		return "keep build failure fix in core/CLI code path and verify release build"
	default:
		return "classify the repeated trace pattern, then add the smallest harness guardrail with a fixture"
	}
}

func overfitRiskForClass(class string) string {
	class = strings.ToLower(class)
	switch class {
	case "deterministic":
		return "medium: deterministic failures justify a guardrail, but keep it fixture-backed"
	case "intermittent":
		return "high: rerun and isolate flake signals before changing harness prompts or hooks"
	case "single_failure_observation":
		return "high: one fail-fast observation is insufficient for broad harness tuning"
	default:
		return "medium: verify on a synthetic trace before changing shared harness behavior"
	}
}

func defaultTraceVerificationCommand(step string) string {
	step = strings.ToLower(step)
	switch {
	case strings.Contains(step, "contract") || strings.Contains(step, "golden"):
		return "go test ./cmd/harness -run Golden -count=1"
	case strings.Contains(step, "policy") || strings.Contains(step, "guard"):
		return "go test ./internal/core -count=1"
	case strings.Contains(step, "build"):
		return "go build -o bin/agent-harness ./cmd/harness"
	default:
		return "go test ./... -count=1"
	}
}

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
	return redactFreeform(items[0])
}

func redactStringSlice(items []string) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = redactFreeform(item)
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
