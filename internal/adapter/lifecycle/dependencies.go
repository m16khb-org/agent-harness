package lifecycle

import (
	"agent-harness/internal/adapter/lifecycle/compact"
	"agent-harness/internal/adapter/lifecycle/docupkeep"
	"agent-harness/internal/adapter/lifecycle/liveapproval"
	"agent-harness/internal/adapter/lifecycle/model"
	"agent-harness/internal/adapter/lifecycle/worktreepath"
	issueopscontract "agent-harness/internal/contract/issueops"
	lifecyclecontract "agent-harness/internal/contract/lifecycle"
	projectdoccontract "agent-harness/internal/contract/projectdoc"
	"agent-harness/internal/domain/nextaction"
	"agent-harness/internal/domain/projectdoc"
	"agent-harness/internal/domain/searchrouting"
)

const ProjectDocsDir = projectdoc.ProjectDocsDir
const ProjectLifecycleSchemaVersion = model.ProjectLifecycleSchemaVersion
const projectLifecycleProfileFile = model.ProjectLifecycleProfileFile
const docUpkeepQueueFile = model.DocUpkeepQueueFile
const compactCapsuleFile = model.CompactCapsuleFile

type ProjectProfile = projectdoccontract.ProjectProfile
type ProjectFingerprint = lifecyclecontract.ProjectFingerprint
type ProjectLifecycleProfile = lifecyclecontract.ProjectLifecycleProfile
type ProjectLifecycleStatePlan = lifecyclecontract.ProjectLifecycleStatePlan
type DocUpkeepAppendResult = lifecyclecontract.DocUpkeepAppendResult
type LifecycleStopReminderResult = lifecyclecontract.LifecycleStopReminderResult
type LifecycleCompactCapsule = lifecyclecontract.LifecycleCompactCapsule
type LifecycleCompactResult = lifecyclecontract.LifecycleCompactResult
type NextActionJudgementTriggerResult = nextaction.NextActionJudgementTriggerResult
type NumberedNextActionsDecisionResult = nextaction.NumberedNextActionsDecisionResult
type NextActionCandidate = nextaction.NextActionCandidate
type NextActionAutoProceedResult = nextaction.NextActionAutoProceedResult

const (
	IssueOpsPhaseProblem     = issueopscontract.IssueOpsPhaseProblem
	IssueOpsPhaseGrill       = issueopscontract.IssueOpsPhaseGrill
	IssueOpsPhasePlan        = issueopscontract.IssueOpsPhasePlan
	IssueOpsPhaseImplement   = issueopscontract.IssueOpsPhaseImplement
	IssueOpsPhaseAISlopClean = issueopscontract.IssueOpsPhaseAISlopClean
	IssueOpsPhasePR          = issueopscontract.IssueOpsPhasePR
	IssueOpsPhaseDone        = issueopscontract.IssueOpsPhaseDone
)

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
	return issueOpsDeps.IssueOpsStateRoot()
}

func StartIssueOps(stateRoot string, req issueopscontract.IssueOpsStartRequest) (issueopscontract.IssueOpsRecord, error) {
	return issueOpsDeps.StartIssueOps(stateRoot, req)
}

func ReadIssueOps(stateRoot, id string) (issueopscontract.IssueOpsRecord, error) {
	return issueOpsDeps.ReadIssueOps(stateRoot, id)
}

func RecordIssueOpsIntent(stateRoot, id string, req issueopscontract.IssueOpsIntentRecordRequest) (issueopscontract.IssueOpsRecord, error) {
	return issueOpsDeps.RecordIssueOpsIntent(stateRoot, id, req)
}

func RecordIssueOpsDesignReview(stateRoot, id string, req issueopscontract.IssueOpsDesignReviewRequest) (issueopscontract.IssueOpsRecord, error) {
	return issueOpsDeps.RecordIssueOpsDesignReview(stateRoot, id, req)
}

func LinkIssueOpsIssue(stateRoot, id, issueURL string) (issueopscontract.IssueOpsRecord, error) {
	return issueOpsDeps.LinkIssueOpsIssue(stateRoot, id, issueURL)
}

func PrepareIssueOpsBranch(stateRoot, id string, req issueopscontract.IssueOpsBranchPrepareRequest) (issueopscontract.IssueOpsRecord, error) {
	return issueOpsDeps.PrepareIssueOpsBranch(stateRoot, id, req)
}

func LinkIssueOpsWorktree(stateRoot, id, worktreePath string) (issueopscontract.IssueOpsRecord, error) {
	return issueOpsDeps.LinkIssueOpsWorktree(stateRoot, id, worktreePath)
}

func LinkIssueOpsPlan(stateRoot, id, planPath string) (issueopscontract.IssueOpsRecord, error) {
	return issueOpsDeps.LinkIssueOpsPlan(stateRoot, id, planPath)
}

func AdvanceIssueOpsPhase(stateRoot, id, to string) (issueopscontract.IssueOpsRecord, error) {
	return issueOpsDeps.AdvanceIssueOpsPhase(stateRoot, id, to)
}

func ActiveIssueOpsCycleForBranch(repo, branch string) (issueopscontract.IssueOpsRecord, bool) {
	return issueOpsDeps.ActiveIssueOpsCycleForBranch(repo, branch)
}

func ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo string) []issueopscontract.IssueOpsRecord {
	return issueOpsDeps.ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo)
}

func IssueOpsPhaseExpectsWorktree(phase issueopscontract.IssueOpsPhase) bool {
	return issueOpsDeps.IssueOpsPhaseExpectsWorktree(phase)
}

func newIssueOpsID(repo, branch string) string {
	return issueOpsDeps.NewIssueOpsID(repo, branch)
}

func writeIssueOps(stateRoot string, record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, error) {
	return issueOpsDeps.WriteIssueOps(stateRoot, record)
}

func AppendDocUpkeepEvent(repoRoot string, event lifecyclecontract.DocUpkeepEvent) (DocUpkeepAppendResult, error) {
	return docupkeep.Append(docUpkeepStore(), repoRoot, event)
}

func ReadPendingDocUpkeepEvents(repoRoot string, limit int) ([]lifecyclecontract.DocUpkeepEvent, ProjectLifecycleStatePlan, error) {
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

func liveApprovalStore() liveapproval.Store {
	toNamespace := func(plan ProjectLifecycleStatePlan) liveapproval.Namespace {
		return liveapproval.Namespace{
			Exists:   plan.Exists,
			Valid:    plan.NamespaceValid,
			RepoRoot: plan.RepoRoot,
			Dir:      plan.ProjectStateDir,
		}
	}
	return liveapproval.Store{
		Resolve: func(repoRoot string) (liveapproval.Namespace, error) {
			plan, err := ValidateProjectLifecycleState(repoRoot)
			return toNamespace(plan), err
		},
		Init: func(repoRoot string) (liveapproval.Namespace, error) {
			plan, err := InitProjectLifecycleState(repoRoot, true)
			return toNamespace(plan), err
		},
		WithLock:  WithKeyLock,
		WriteJSON: writeJSONAtomic,
	}
}

func worktreeGuardEditTargets(req lifecyclecontract.HookToolUseLifecycleRequest) []string {
	targets := []string{}
	base := hookRequestPathBase(req)
	for _, path := range req.Paths {
		if target := resolveHookTargetPath(base, path); target != "" {
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 && searchrouting.IsShellTool(req.Tool) {
		for _, path := range shellCommandWorktreeGuardPaths(base, req.Command) {
			if target := resolveHookTargetPath(base, path); target != "" {
				targets = append(targets, target)
			}
		}
	}
	if len(targets) == 0 {
		if base != "" {
			targets = append(targets, base)
		}
	}
	return targets
}

func hookRequestPathBase(req lifecyclecontract.HookToolUseLifecycleRequest) string {
	if searchrouting.IsShellTool(req.Tool) {
		if workdir, ok := req.ToolInput["workdir"].(string); ok {
			if root := resolveHookTargetPath(req.CWD, workdir); root != "" {
				return root
			}
		}
	}
	if cwd := cleanAbsPath(req.CWD); cwd != "" {
		return cwd
	}
	return cleanAbsPath(req.Repo)
}

func shellCommandWorktreeGuardPaths(repo, command string) []string {
	return worktreepath.ShellCommandGuardPaths(repo, command)
}

func gitBranchFromHead(repo string) string {
	return worktreepath.GitBranchFromHead(repo)
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
