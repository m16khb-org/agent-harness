package hookprompt

import (
	"agent-harness/internal/core/lifecycle"
	"agent-harness/internal/core/projectdoc"
	"agent-harness/internal/core/projectdocs"
)

type DocUpkeepEvent = lifecycle.DocUpkeepEvent
type StopNextActionRelayRecord = lifecycle.StopNextActionRelayRecord
type IssueOpsRecord = lifecycle.IssueOpsRecord
type ProjectProfile = projectdocs.ProjectProfile
type ProjectDocCatalogEntry = projectdoc.ProjectDocCatalogEntry

func ResolveProjectLifecycleState(repoRoot string) (lifecycle.ProjectLifecycleStatePlan, error) {
	return lifecycle.ResolveProjectLifecycleState(repoRoot)
}

func ReadPendingDocUpkeepEvents(repoRoot string, limit int) ([]DocUpkeepEvent, lifecycle.ProjectLifecycleStatePlan, error) {
	return lifecycle.ReadPendingDocUpkeepEvents(repoRoot, limit)
}

func ReadStopNextActionRelay(repoRoot string) (StopNextActionRelayRecord, bool) {
	return lifecycle.ReadStopNextActionRelay(repoRoot)
}

func ApproveCodexKubectlLiveAccess(repo, host, sessionID, prompt string) (bool, string) {
	result := lifecycle.ApproveCodexKubectlLiveAccess(repo, host, sessionID, prompt)
	return result.Handled, result.AdditionalContext
}

func ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo string) []IssueOpsRecord {
	return lifecycle.ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo)
}

func IssueOpsPhaseExpectsWorktree(phase lifecycle.IssueOpsPhase) bool {
	return lifecycle.IssueOpsPhaseExpectsWorktree(phase)
}

func DiscoverProjectDocs(repoRoot string) []ProjectDocCatalogEntry {
	return projectdoc.DiscoverProjectDocs(repoRoot)
}

func FormatProjectDocCatalog(entries []ProjectDocCatalogEntry) string {
	return projectdoc.FormatProjectDocCatalog(entries)
}
