package statuscli

import (
	"issueops/internal/adapter/preflight"
	"issueops/internal/adapter/projectdocs"
)

// production wiring과 같은 구현을 설치한다. Configure를 호출하지 않는 테스트도
// 실제 구현으로 동작을 검증한다. fitness graph는 test import를 수집하지 않으므로
// 여기서는 concrete를 써도 된다.
func init() {
	d := deps
	d.GitPreflight = preflight.GitPreflight
	d.AnalyzeProjectSignals = projectdocs.AnalyzeProjectSignals
	Configure(d)
}
