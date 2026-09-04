package mcpcli

import (
	"context"

	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// 원격 발행 핸들러 묶음이다. 어댑터의 이름 붙은 타입 대신 같은 시그니처를 여기서
// 선언해 CLI가 어댑터를 알지 않게 한다 — Go에서 두 형태는 할당 호환이다.
type PublicationHandlers struct {
	Create    func(context.Context, string, issueopscontract.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error)
	Reconcile func(context.Context, string, issueopscontract.ExecutionReconcileRequest) (issueopscontract.ExecutionReconcileResult, error)
}
