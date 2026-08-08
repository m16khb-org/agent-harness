package basiccli

import (
	projectbootstrapcontract "agent-harness/internal/contract/projectbootstrap"
	"errors"
)

// 프로젝트 문서 부트스트랩은 저장소를 읽고 쓰는 I/O다. CLI는 그 구현을 모르고
// composition root가 주입한 함수만 호출한다.
var bootstrapProjectDocs = func(projectbootstrapcontract.ProjectDocsBootstrapRequest) (projectbootstrapcontract.ProjectDocsBootstrapResult, error) {
	return projectbootstrapcontract.ProjectDocsBootstrapResult{}, errors.New("project bootstrap is not configured")
}

// ConfigureProjectBootstrap는 composition root가 실제 구현을 꽂는 진입점이다.
func ConfigureProjectBootstrap(bootstrap func(projectbootstrapcontract.ProjectDocsBootstrapRequest) (projectbootstrapcontract.ProjectDocsBootstrapResult, error)) {
	if bootstrap != nil {
		bootstrapProjectDocs = bootstrap
	}
}
