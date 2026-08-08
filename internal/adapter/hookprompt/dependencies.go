package hookprompt

import (
	issueopscontract "agent-harness/internal/contract/issueops"
	lifecyclecontract "agent-harness/internal/contract/lifecycle"
	projectdoccontract "agent-harness/internal/contract/projectdoc"
	"agent-harness/internal/domain/projectdoc"
)

type ProjectProfile = projectdoccontract.ProjectProfile
type ProjectDocCatalogEntry = projectdoc.ProjectDocCatalogEntry

// lifecycle 연산은 사용자 상태를 읽고 쓰는 I/O다. hookprompt는 그 구현을 모르고
// composition root가 주입한 함수만 호출한다. 기본값은 프롬프트를 실패시키지 않는
// 중립 응답이다.
var lifecycleDeps = LifecycleDeps{
	ResolveProjectLifecycleState: func(string) (lifecyclecontract.ProjectLifecycleStatePlan, error) {
		return lifecyclecontract.ProjectLifecycleStatePlan{}, nil
	},
	ReadPendingDocUpkeepEvents: func(string, int) ([]lifecyclecontract.DocUpkeepEvent, lifecyclecontract.ProjectLifecycleStatePlan, error) {
		return nil, lifecyclecontract.ProjectLifecycleStatePlan{}, nil
	},
	ReadStopNextActionRelay: func(string) (lifecyclecontract.StopNextActionRelayRecord, bool) {
		return lifecyclecontract.StopNextActionRelayRecord{}, false
	},
	ApproveCodexKubectlLiveAccess:             func(string, string, string, string) (bool, string) { return false, "" },
	ActiveIssueOpsLinkedWorktreeCyclesForRepo: func(string) []issueopscontract.IssueOpsRecord { return nil },
	IssueOpsPhaseExpectsWorktree:              func(issueopscontract.IssueOpsPhase) bool { return false },
}

// LifecycleDeps는 composition root가 실제 lifecycle 어댑터를 꽂는 진입점이다.
type LifecycleDeps struct {
	ResolveProjectLifecycleState              func(string) (lifecyclecontract.ProjectLifecycleStatePlan, error)
	ReadPendingDocUpkeepEvents                func(string, int) ([]lifecyclecontract.DocUpkeepEvent, lifecyclecontract.ProjectLifecycleStatePlan, error)
	ReadStopNextActionRelay                   func(string) (lifecyclecontract.StopNextActionRelayRecord, bool)
	ApproveCodexKubectlLiveAccess             func(repo, host, sessionID, prompt string) (bool, string)
	ActiveIssueOpsLinkedWorktreeCyclesForRepo func(string) []issueopscontract.IssueOpsRecord
	IssueOpsPhaseExpectsWorktree              func(issueopscontract.IssueOpsPhase) bool
}

func ConfigureLifecycle(deps LifecycleDeps) {
	if deps.ResolveProjectLifecycleState != nil {
		lifecycleDeps.ResolveProjectLifecycleState = deps.ResolveProjectLifecycleState
	}
	if deps.ReadPendingDocUpkeepEvents != nil {
		lifecycleDeps.ReadPendingDocUpkeepEvents = deps.ReadPendingDocUpkeepEvents
	}
	if deps.ReadStopNextActionRelay != nil {
		lifecycleDeps.ReadStopNextActionRelay = deps.ReadStopNextActionRelay
	}
	if deps.ApproveCodexKubectlLiveAccess != nil {
		lifecycleDeps.ApproveCodexKubectlLiveAccess = deps.ApproveCodexKubectlLiveAccess
	}
	if deps.ActiveIssueOpsLinkedWorktreeCyclesForRepo != nil {
		lifecycleDeps.ActiveIssueOpsLinkedWorktreeCyclesForRepo = deps.ActiveIssueOpsLinkedWorktreeCyclesForRepo
	}
	if deps.IssueOpsPhaseExpectsWorktree != nil {
		lifecycleDeps.IssueOpsPhaseExpectsWorktree = deps.IssueOpsPhaseExpectsWorktree
	}
}

func ResolveProjectLifecycleState(repoRoot string) (lifecyclecontract.ProjectLifecycleStatePlan, error) {
	return lifecycleDeps.ResolveProjectLifecycleState(repoRoot)
}

func ReadPendingDocUpkeepEvents(repoRoot string, limit int) ([]lifecyclecontract.DocUpkeepEvent, lifecyclecontract.ProjectLifecycleStatePlan, error) {
	return lifecycleDeps.ReadPendingDocUpkeepEvents(repoRoot, limit)
}

func ReadStopNextActionRelay(repoRoot string) (lifecyclecontract.StopNextActionRelayRecord, bool) {
	return lifecycleDeps.ReadStopNextActionRelay(repoRoot)
}

func ApproveCodexKubectlLiveAccess(repo, host, sessionID, prompt string) (bool, string) {
	return lifecycleDeps.ApproveCodexKubectlLiveAccess(repo, host, sessionID, prompt)
}

func ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo string) []issueopscontract.IssueOpsRecord {
	return lifecycleDeps.ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo)
}

func IssueOpsPhaseExpectsWorktree(phase issueopscontract.IssueOpsPhase) bool {
	return lifecycleDeps.IssueOpsPhaseExpectsWorktree(phase)
}
