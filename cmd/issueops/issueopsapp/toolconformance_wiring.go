package issueopsapp

import (
	"issueops/cmd/issueops/contractcli"
	"issueops/internal/adapter/toolconformance"
)

// contractcli는 fixture 저장소 구현을 알지 않는다. 어댑터를 아는 곳은
// composition root 하나뿐이다.
func configureToolConformance() {
	contractcli.ConfigureToolConformance(contractcli.ToolConformanceDeps{
		LoadManifest:          toolconformance.LoadManifest,
		LoadRegressionFixture: toolconformance.LoadRegressionFixture,
		ReplayRegression:      toolconformance.ReplayRegression,
	})
}
