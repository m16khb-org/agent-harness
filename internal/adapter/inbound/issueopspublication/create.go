package issueopspublication

import (
	"context"

	issueopscontract "agent-harness/internal/contract/issueops"

	publicationapp "agent-harness/internal/application/issueopspublication"
	publicationcontract "agent-harness/internal/contract/issueopspublication"
	"agent-harness/internal/port"
)

type createService interface {
	Create(context.Context, publicationcontract.CreateCommand) (publicationcontract.ProviderCreateResult, error)
}

var _ createService = (*publicationapp.CreateService)(nil)

type CreateHandler struct{ service createService }

// 반환 타입은 어댑터의 이름 붙은 핸들러 타입 대신 같은 시그니처를 직접 쓴다.
// Go에서 두 형태는 할당 호환이므로 소비자는 그대로 동작하고, inbound 어댑터는
// issueops 어댑터를 알 필요가 없어진다.
func NewCreateHandler(service createService) func(context.Context, string, issueopscontract.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	return CreateHandler{service: service}.Handle
}

func (h CreateHandler) Handle(ctx context.Context, _ string, request issueopscontract.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	if h.service == nil {
		return port.IssueProviderCreatePullRequestResult{}, issueopscontract.ErrRemotePullRequestCreateHandlerUnavailable
	}
	result, err := h.service.Create(ctx, publicationcontract.CreateCommand{
		ID: request.ID, Provider: request.Provider, Title: request.Title, Body: request.Body,
		Head: request.Head, Base: request.Base,
		Labels: cloneStrings(request.Labels), Assignees: cloneStrings(request.Assignees),
		ExpectedGeneration: request.ExpectedGeneration, Actor: publicationActor(request.Actor),
		CWD: request.CWD, Confirm: request.Confirm,
	})
	return port.IssueProviderCreatePullRequestResult{
		OK: result.OK, URL: result.URL, Number: result.Number, Preview: result.Preview,
	}, err
}

func publicationActor(actor issueopscontract.NativeActor) publicationcontract.Actor {
	result := publicationcontract.Actor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.SessionProcess != nil {
		result.SessionProcess = &publicationcontract.ProcessReceipt{
			PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable,
		}
	}
	if actor.ProcessAncestry != nil {
		result.ProcessAncestry = make([]publicationcontract.ProcessReceipt, len(actor.ProcessAncestry))
		for index, receipt := range actor.ProcessAncestry {
			result.ProcessAncestry[index] = publicationcontract.ProcessReceipt{
				PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable,
			}
		}
	}
	return result
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}
