// Package mcp는 mcp capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package mcp

type ConformanceProbeConfig struct {
	FixtureID          string
	ProbeTool          string
	Schema             map[string]any
	SchemaSHA          string
	ExpectedArguments  map[string]any
	ResultPath         string
	RunToken           string
	ProductionDispatch func()
}
