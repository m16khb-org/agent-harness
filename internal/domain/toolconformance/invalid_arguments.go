package toolconformance

import toolconformancecontract "issueops/internal/contract/toolconformance"

// InvalidToolArgumentsResult는 진단 목록을 호스트 응답 형태로 정규화한다.
// 순수 변환이므로 domain 계층이 소유한다.
func InvalidToolArgumentsResult(tool string, diagnostics []toolconformancecontract.Diagnostic) map[string]any {
	return map[string]any{
		"ok":      false,
		"isError": true,
		"error": map[string]any{
			"code":        "invalid_tool_arguments",
			"tool":        tool,
			"diagnostics": append([]toolconformancecontract.Diagnostic(nil), diagnostics...),
		},
	}
}
