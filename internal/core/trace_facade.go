package core

import coretrace "agent-harness/internal/core/trace"

const TraceAnalysisKind = coretrace.TraceAnalysisKind

type TraceAnalyzeRequest = coretrace.TraceAnalyzeRequest
type TraceAnalyzeResult = coretrace.TraceAnalyzeResult
type TraceAnalysisFinding = coretrace.TraceAnalysisFinding

func TraceAnalyze(req TraceAnalyzeRequest) (TraceAnalyzeResult, error) {
	return coretrace.TraceAnalyze(req)
}
