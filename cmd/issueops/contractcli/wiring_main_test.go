package contractcli

import (
	"issueops/internal/adapter/toolconformance"
	"os"
	"testing"
)

// 프로덕션에서는 issueopsapp이 주입한다. 계약 CLI 테스트는 실제 fixture 저장소를
// 읽으므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {
	ConfigureToolConformance(ToolConformanceDeps{
		LoadManifest:          toolconformance.LoadManifest,
		LoadRegressionFixture: toolconformance.LoadRegressionFixture,
		ReplayRegression:      toolconformance.ReplayRegression,
	})
	os.Exit(m.Run())
}
