package hookprompt

import (
	"agent-harness/internal/adapter/lifecycle"
	issueopscontract "agent-harness/internal/contract/issueops"
	lifecyclecontract "agent-harness/internal/contract/lifecycle"
	projectdoccontract "agent-harness/internal/contract/projectdoc"
	"agent-harness/internal/domain/projectdoc"
)

type ProjectProfile = projectdoccontract.ProjectProfile
type ProjectDocCatalogEntry = projectdoc.ProjectDocCatalogEntry

func ResolveProjectLifecycleState(repoRoot string) (lifecycle.ProjectLifecycleStatePlan, error) {
	return lifecycle.ResolveProjectLifecycleState(repoRoot)
}

func ReadPendingDocUpkeepEvents(repoRoot string, limit int) ([]lifecyclecontract.DocUpkeepEvent, lifecycle.ProjectLifecycleStatePlan, error) {
	return lifecycle.ReadPendingDocUpkeepEvents(repoRoot, limit)
}

func ReadStopNextActionRelay(repoRoot string) (lifecyclecontract.StopNextActionRelayRecord, bool) {
	return lifecycle.ReadStopNextActionRelay(repoRoot)
}

func ApproveCodexKubectlLiveAccess(repo, host, sessionID, prompt string) (bool, string) {
	result := lifecycle.ApproveCodexKubectlLiveAccess(repo, host, sessionID, prompt)
	return result.Handled, result.AdditionalContext
}

func ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo string) []issueopscontract.IssueOpsRecord {
	return lifecycle.ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo)
}

func IssueOpsPhaseExpectsWorktree(phase issueopscontract.IssueOpsPhase) bool {
	return lifecycle.IssueOpsPhaseExpectsWorktree(phase)
}
