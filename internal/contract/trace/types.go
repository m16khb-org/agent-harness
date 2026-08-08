// Package trace는 trace capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package trace

import failurecause "agent-harness/internal/contract/failurecause"

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
	FailureClass         string                  `json:"failure_class"`
	FailureCause         failurecause.Cause      `json:"failure_cause"`
	FailureCauseEvidence []failurecause.Evidence `json:"failure_cause_evidence"`
	RecurringPattern     string                  `json:"recurring_pattern"`
	ProposedKnob         string                  `json:"proposed_knob"`
	OverfitRisk          string                  `json:"overfit_risk"`
	VerificationCommand  string                  `json:"verification_command"`
}
