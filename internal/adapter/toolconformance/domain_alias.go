package toolconformance

import toolconformancedomain "agent-harness/internal/domain/toolconformance"

// 스키마 판정은 순수 규칙이므로 domain 계층이 소유한다. 어댑터는 재노출만 한다.
var (
	InvalidToolArgumentsResult = toolconformancedomain.InvalidToolArgumentsResult
	Classify                   = toolconformancedomain.Classify
	ClosedProjection           = toolconformancedomain.ClosedProjection
	Validate                   = toolconformancedomain.Validate
	SortDiagnostics            = toolconformancedomain.SortDiagnostics
	sortDiagnostics            = toolconformancedomain.SortDiagnostics
	CanonicalSchemaSHA256      = toolconformancedomain.CanonicalSchemaSHA256
)
