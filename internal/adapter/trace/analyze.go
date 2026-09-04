package trace

import (
	"fmt"
	tracecontract "issueops/internal/contract/trace"
	"strings"
)

const TraceAnalysisKind = "trace_analysis"

func TraceAnalyze(req tracecontract.TraceAnalyzeRequest) (tracecontract.TraceAnalyzeResult, error) {
	input := strings.TrimSpace(req.Input)
	result := tracecontract.TraceAnalyzeResult{
		OK:         false,
		Kind:       TraceAnalysisKind,
		Input:      input,
		TraceTypes: []string{},
		Findings:   []tracecontract.TraceAnalysisFinding{},
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
