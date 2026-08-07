package trace

import (
	"encoding/json"
	"strings"
)

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
			findings = append(findings, docUpkeepFindings([]docUpkeepEvent{event})...)
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
