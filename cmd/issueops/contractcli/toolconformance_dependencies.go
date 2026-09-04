package contractcli

import (
	toolconformancecontract "issueops/internal/contract/toolconformance"
)

// fixture 로딩과 리플레이는 파일시스템을 읽는 I/O다. contractcli는 그 구현을
// 모르고 composition root가 주입한 함수만 호출한다.
var (
	loadManifest = func([]toolconformancecontract.ToolDescriptor) ([]toolconformancecontract.Fixture, []toolconformancecontract.BaselineCase, error) {
		return nil, nil, nil
	}
	loadRegressionFixture = func(string) (toolconformancecontract.RegressionFixture, error) {
		return toolconformancecontract.RegressionFixture{}, nil
	}
	replayRegression = func(toolconformancecontract.RegressionFixture, []toolconformancecontract.ToolDescriptor, string) (toolconformancecontract.ReplayResult, error) {
		return toolconformancecontract.ReplayResult{}, nil
	}
)

// ToolConformanceDeps는 composition root가 실제 구현을 꽂는 진입점이다.
type ToolConformanceDeps struct {
	LoadManifest          func([]toolconformancecontract.ToolDescriptor) ([]toolconformancecontract.Fixture, []toolconformancecontract.BaselineCase, error)
	LoadRegressionFixture func(string) (toolconformancecontract.RegressionFixture, error)
	ReplayRegression      func(toolconformancecontract.RegressionFixture, []toolconformancecontract.ToolDescriptor, string) (toolconformancecontract.ReplayResult, error)
}

func ConfigureToolConformance(deps ToolConformanceDeps) {
	if deps.LoadManifest != nil {
		loadManifest = deps.LoadManifest
	}
	if deps.LoadRegressionFixture != nil {
		loadRegressionFixture = deps.LoadRegressionFixture
	}
	if deps.ReplayRegression != nil {
		replayRegression = deps.ReplayRegression
	}
}
