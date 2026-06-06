package core

import (
	"fmt"
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
