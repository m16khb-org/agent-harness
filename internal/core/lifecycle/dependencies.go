package lifecycle

import (
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/lifecycle/compact"
	"agent-harness/internal/core/lifecycle/docupkeep"
	"agent-harness/internal/core/lifecycle/model"
	"agent-harness/internal/core/lifecycle/worktreepath"
	"agent-harness/internal/core/nextaction"
	"agent-harness/internal/core/projectdoc"
	"agent-harness/internal/core/projectdocs"
	"agent-harness/internal/core/searchrouting"
	"agent-harness/internal/core/state"
)

const ProjectDocsDir = projectdoc.ProjectDocsDir
const ProjectLifecycleSchemaVersion = model.ProjectLifecycleSchemaVersion
const projectLifecycleProfileFile = model.ProjectLifecycleProfileFile
const docUpkeepQueueFile = model.DocUpkeepQueueFile
const compactCapsuleFile = model.CompactCapsuleFile
const stopNextActionRelayFile = model.StopNextActionRelayFile

type ProjectProfile = projectdocs.ProjectProfile
type ProjectFingerprint = model.ProjectFingerprint
type ProjectLifecycleProfile = model.ProjectLifecycleProfile
type ProjectLifecycleStatePlan = model.ProjectLifecycleStatePlan
type DocUpkeepEvent = model.DocUpkeepEvent
type DocUpkeepAppendResult = model.DocUpkeepAppendResult
type HookToolUseLifecycleRequest = model.HookToolUseLifecycleRequest
type HookToolUseLifecycleResult = model.HookToolUseLifecycleResult
type HookPreToolUseDecisionResult = model.HookPreToolUseDecisionResult
type LifecycleStopReminderResult = model.LifecycleStopReminderResult
type StopNextActionRelayRecord = model.StopNextActionRelayRecord
type StopNextActionRelayResult = model.StopNextActionRelayResult
type LifecycleCompactCapsule = model.LifecycleCompactCapsule
type LifecycleCompactResult = model.LifecycleCompactResult
type NextActionJudgementTriggerResult = nextaction.NextActionJudgementTriggerResult
type NumberedNextActionsDecisionResult = nextaction.NumberedNextActionsDecisionResult
type NextActionCandidate = nextaction.NextActionCandidate
type NextActionAutoProceedResult = nextaction.NextActionAutoProceedResult

type IssueOpsRecord = issueops.IssueOpsRecord
type IssueOpsStartRequest = issueops.IssueOpsStartRequest
type IssueOpsIntentRecordRequest = issueops.IssueOpsIntentRecordRequest
type IssueOpsDesignReviewRequest = issueops.IssueOpsDesignReviewRequest
type IssueOpsBranchPrepareRequest = issueops.IssueOpsBranchPrepareRequest
type IssueOpsRemoteArtifactVerification = issueops.IssueOpsRemoteArtifactVerification
type IssueOpsPhase = issueops.IssueOpsPhase

const (
	IssueOpsPhaseProblem     = issueops.IssueOpsPhaseProblem
	IssueOpsPhaseGrill       = issueops.IssueOpsPhaseGrill
	IssueOpsPhasePlan        = issueops.IssueOpsPhasePlan
	IssueOpsPhaseImplement   = issueops.IssueOpsPhaseImplement
	IssueOpsPhaseAISlopClean = issueops.IssueOpsPhaseAISlopClean
	IssueOpsPhasePR          = issueops.IssueOpsPhasePR
	IssueOpsPhaseDone        = issueops.IssueOpsPhaseDone
)

func StateDir() string {
	return state.StateDir()
}

func BuildNumberedNextActionsDecision(message string, enforce bool, source string) NumberedNextActionsDecisionResult {
	return nextaction.BuildNumberedNextActionsDecision(message, enforce, source)
}

func BuildNextActionJudgementTrigger(message string) NextActionJudgementTriggerResult {
	return nextaction.BuildNextActionJudgementTrigger(message)
}

func BuildNextActionJudgementRelayReason(trigger NextActionJudgementTriggerResult) string {
	return nextaction.BuildJudgementRelayReason(trigger)
}

func IsNoAutoProceedJudgement(message string) bool {
	return nextaction.IsNoAutoProceedJudgement(message)
}

func EvaluateNextActionAutoProceed(message string, threshold float64) NextActionAutoProceedResult {
	return nextaction.EvaluateNextActionAutoProceed(message, threshold)
}

func IssueOpsStateRoot() string {
	return issueops.IssueOpsStateRoot()
}

func StartIssueOps(stateRoot string, req IssueOpsStartRequest) (IssueOpsRecord, error) {
	return issueops.StartIssueOps(stateRoot, req)
}

func ReadIssueOps(stateRoot, id string) (IssueOpsRecord, error) {
	return issueops.ReadIssueOps(stateRoot, id)
}

func RecordIssueOpsIntent(stateRoot, id string, req IssueOpsIntentRecordRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsIntent(stateRoot, id, req)
}

func RecordIssueOpsDesignReview(stateRoot, id string, req IssueOpsDesignReviewRequest) (IssueOpsRecord, error) {
	return issueops.RecordIssueOpsDesignReview(stateRoot, id, req)
}

func LinkIssueOpsIssue(stateRoot, id, issueURL string) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsIssue(stateRoot, id, issueURL)
}

func PrepareIssueOpsBranch(stateRoot, id string, req IssueOpsBranchPrepareRequest) (IssueOpsRecord, error) {
	return issueops.PrepareIssueOpsBranch(stateRoot, id, req)
}

func LinkIssueOpsWorktree(stateRoot, id, worktreePath string) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsWorktree(stateRoot, id, worktreePath)
}

func LinkIssueOpsPlan(stateRoot, id, planPath string) (IssueOpsRecord, error) {
	return issueops.LinkIssueOpsPlan(stateRoot, id, planPath)
}

func AdvanceIssueOpsPhase(stateRoot, id, to string) (IssueOpsRecord, error) {
	return issueops.AdvanceIssueOpsPhase(stateRoot, id, to)
}

func ActiveIssueOpsCycleForBranch(repo, branch string) (IssueOpsRecord, bool) {
	return issueops.ActiveIssueOpsCycleForBranch(repo, branch)
}

func ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo string) []IssueOpsRecord {
	return issueops.ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo)
}

func ActiveIssueOpsSupervisedHandoffCyclesForRepo(repo string) []IssueOpsRecord {
	return issueops.ActiveIssueOpsSupervisedHandoffCyclesForRepo(repo)
}

func IssueOpsPhaseExpectsWorktree(phase IssueOpsPhase) bool {
	return issueops.IssueOpsPhaseExpectsWorktree(phase)
}

func issueOpsCycleWorktreeMissing(record IssueOpsRecord) bool {
	return issueops.IssueOpsCycleWorktreeMissing(record)
}

func readIssueOpsSession(repo string) (issueops.SessionBinding, error) {
	return issueops.ReadIssueOpsSession(repo)
}

func listIssueOpsSessionBindings(repo string) ([]issueops.SessionBinding, error) {
	return issueops.ListIssueOpsSessionBindings(repo)
}

func activeSessionCycleID(repo string) string {
	return issueops.ActiveSessionCycleID(repo)
}

func validateIssueOpsIssueBranch(branch string) error {
	return issueops.ValidateIssueOpsIssueBranch(branch)
}

func newIssueOpsID(repo, branch string) string {
	return issueops.NewIssueOpsID(repo, branch)
}

func writeIssueOps(stateRoot string, record IssueOpsRecord) (IssueOpsRecord, error) {
	return issueops.WriteIssueOps(stateRoot, record)
}

func AppendDocUpkeepEvent(repoRoot string, event DocUpkeepEvent) (DocUpkeepAppendResult, error) {
	return docupkeep.Append(docUpkeepStore(), repoRoot, event)
}

func ReadPendingDocUpkeepEvents(repoRoot string, limit int) ([]DocUpkeepEvent, ProjectLifecycleStatePlan, error) {
	return docupkeep.ReadPending(docUpkeepStore(), repoRoot, limit)
}

func BuildLifecyclePreCompactCapsule(repo string) LifecycleCompactResult {
	return compact.BuildPreCompactCapsule(compactStore(), repo)
}

func BuildLifecyclePostCompactReminder(repo string) LifecycleCompactResult {
	return compact.BuildPostCompactReminder(compactStore(), repo)
}

func docUpkeepStore() docupkeep.Store {
	return docupkeep.Store{
		Validate: ValidateProjectLifecycleState,
		Init: func(repoRoot string, confirm bool) (ProjectLifecycleStatePlan, error) {
			return InitProjectLifecycleState(repoRoot, confirm)
		},
	}
}

func compactStore() compact.Store {
	return compact.Store{
		ReadPending: ReadPendingDocUpkeepEvents,
		Validate:    ValidateProjectLifecycleState,
		WriteJSON:   writeJSONAtomic,
	}
}

func normalizeTargetDocs(docs []string) []string {
	return docupkeep.NormalizeTargetDocs(docs)
}

func worktreeGuardTargets(req HookToolUseLifecycleRequest) []string {
	targets := []string{}
	if repo := cleanAbsPath(req.Repo); repo != "" {
		targets = append(targets, repo)
	}
	for _, path := range req.Paths {
		if target := resolveHookTargetPath(req.Repo, path); target != "" {
			targets = append(targets, target)
		}
	}
	return targets
}

func worktreeGuardEditTargets(req HookToolUseLifecycleRequest) []string {
	targets := []string{}
	for _, path := range req.Paths {
		if target := resolveHookTargetPath(req.Repo, path); target != "" {
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 && searchrouting.IsShellTool(req.Tool) {
		for _, path := range shellCommandWorktreeGuardPaths(req.Repo, req.Command) {
			if target := resolveHookTargetPath(req.Repo, path); target != "" {
				targets = append(targets, target)
			}
		}
	}
	if len(targets) == 0 {
		if repo := cleanAbsPath(req.Repo); repo != "" {
			targets = append(targets, repo)
		}
	}
	return targets
}

func shellCommandWorktreeGuardPaths(repo, command string) []string {
	return worktreepath.ShellCommandGuardPaths(repo, command)
}

func issueOpsWorktreePreparationCommand(command string) bool {
	return worktreepath.IssueOpsPreparationCommand(command)
}

func gitBranchFromHead(repo string) string {
	return worktreepath.GitBranchFromHead(repo)
}

func isInsideWorktreesPath(target string) bool {
	return worktreepath.IsInsideWorktreesPath(target)
}

func resolveHookTargetPath(repo, path string) string {
	return worktreepath.ResolveHookTargetPath(repo, path)
}

func cleanAbsPath(path string) string {
	return worktreepath.CleanAbs(path)
}

func pathWithin(path, root string) bool {
	return worktreepath.Within(path, root)
}
