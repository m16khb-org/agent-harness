package issueopscli

import (
	"context"
	"errors"

	issueopscontract "issueops/internal/contract/issueops"
	orphancontract "issueops/internal/contract/issueopsorphancleanup"
	corehealth "issueops/internal/domain/operationalhealth"
)

var errOrphanNotConfigured = errors.New("issueops orphan cleanup is not configured")

// OrphanDependencies는 CLI가 공급하는 읽기 전용 관측면이다. 어댑터의 이름 붙은
// 타입 대신 같은 필드를 여기서 선언해 CLI가 어댑터를 알지 않게 한다.
type OrphanDependencies struct {
	Collect      func(context.Context, string) (corehealth.Snapshot, error)
	VerifyMerged func(issueopscontract.IssueOpsRemoteArtifactVerification) error
}

// 고아 워크트리 정리는 파일시스템과 원격 제공자를 다루는 I/O다. CLI는 그 구현을
// 모르고 composition root가 주입한 함수만 호출한다.
var (
	orphanPreview = func(context.Context, orphancontract.Request, OrphanDependencies) (orphancontract.Result, error) {
		return orphancontract.Result{}, errOrphanNotConfigured
	}
	orphanApply = func(context.Context, orphancontract.Request, orphancontract.ApplyRequest, OrphanDependencies) (orphancontract.Result, error) {
		return orphancontract.Result{}, errOrphanNotConfigured
	}
)

// OrphanCleanupDeps는 composition root가 실제 어댑터를 꽂는 진입점이다.
type OrphanCleanupDeps struct {
	Preview func(context.Context, orphancontract.Request, OrphanDependencies) (orphancontract.Result, error)
	Apply   func(context.Context, orphancontract.Request, orphancontract.ApplyRequest, OrphanDependencies) (orphancontract.Result, error)
}

func ConfigureOrphanCleanup(deps OrphanCleanupDeps) {
	if deps.Preview != nil {
		orphanPreview = deps.Preview
	}
	if deps.Apply != nil {
		orphanApply = deps.Apply
	}
}
