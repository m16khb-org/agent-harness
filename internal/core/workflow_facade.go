package core

import (
	"agent-harness/internal/core/hookprompt"
	"agent-harness/internal/core/lifecycle"
	"agent-harness/internal/core/nextaction"
	"agent-harness/internal/core/policy"
	"agent-harness/internal/core/projectdocs"
	coreworker "agent-harness/internal/core/worker"
)

type HookUserPromptRequest = hookprompt.HookUserPromptRequest
type HookUserPromptHint = hookprompt.HookUserPromptHint
type HookUserPromptResult = hookprompt.HookUserPromptResult
type HookRoutingRule = hookprompt.HookRoutingRule
type ProjectDocCatalogContext = hookprompt.ProjectDocCatalogContext

const (
	hintPriorityRequired  = hookprompt.PriorityRequired
	hintPriorityConsider  = hookprompt.PriorityConsider
	hintPriorityRoute     = hookprompt.PriorityRoute
	hintPriorityAction    = hookprompt.PriorityAction
	hintPrioritySecondary = hookprompt.PrioritySecondary
)

func BuildUserPromptMCPHints(req HookUserPromptRequest) HookUserPromptResult {
	return hookprompt.BuildUserPromptMCPHints(req)
}

func BuildProjectDocCatalogContext(repo string) ProjectDocCatalogContext {
	return hookprompt.BuildProjectDocCatalogContext(repo)
}

func RenderUserPromptUserView(result HookUserPromptResult) string {
	return hookprompt.RenderUserPromptUserView(result)
}

func RenderUserPromptCodexContext(result HookUserPromptResult) string {
	return hookprompt.RenderUserPromptCodexContext(result)
}

func renderHookMCPHintContext(hints []HookUserPromptHint, pendingUpkeep []DocUpkeepEvent, profile *ProjectProfile, catalog string) string {
	return hookprompt.RenderHookMCPHintContext(hints, pendingUpkeep, profile, catalog)
}

func appendCompactPendingUpkeep(parts *[]string, events []DocUpkeepEvent) {
	hookprompt.AppendCompactPendingUpkeep(parts, events)
}

func fallbackHintPriority(h HookUserPromptHint) string {
	return hookprompt.FallbackHintPriority(h)
}

func compactHintLabel(h HookUserPromptHint) string {
	return hookprompt.CompactHintLabel(h)
}

func containsAnySlice(s string, needles []string) bool {
	return hookprompt.ContainsAnySlice(s, needles)
}

func containsAny(s string, needles ...string) bool {
	return hookprompt.ContainsAny(s, needles...)
}

const ProjectLifecycleSchemaVersion = lifecycle.ProjectLifecycleSchemaVersion

type ProjectFingerprint = lifecycle.ProjectFingerprint
type ProjectLifecycleProfile = lifecycle.ProjectLifecycleProfile
type ProjectLifecycleStatePlan = lifecycle.ProjectLifecycleStatePlan
type DocUpkeepEvent = lifecycle.DocUpkeepEvent
type DocUpkeepAppendResult = lifecycle.DocUpkeepAppendResult
type HookToolUseLifecycleRequest = lifecycle.HookToolUseLifecycleRequest
type HookToolUseLifecycleResult = lifecycle.HookToolUseLifecycleResult
type HookPreToolUseDecisionResult = lifecycle.HookPreToolUseDecisionResult
type LifecycleStopReminderResult = lifecycle.LifecycleStopReminderResult
type StopNextActionRelayRecord = lifecycle.StopNextActionRelayRecord
type StopNextActionRelayResult = lifecycle.StopNextActionRelayResult
type LifecycleCompactCapsule = lifecycle.LifecycleCompactCapsule
type LifecycleCompactResult = lifecycle.LifecycleCompactResult

func BuildLifecyclePreToolUseDecision(req HookToolUseLifecycleRequest) HookPreToolUseDecisionResult {
	return lifecycle.BuildLifecyclePreToolUseDecision(req)
}

func RecordLifecycleToolUse(req HookToolUseLifecycleRequest) (HookToolUseLifecycleResult, error) {
	return lifecycle.RecordLifecycleToolUse(req)
}

func BuildLifecycleStopReminder(repo string) LifecycleStopReminderResult {
	return lifecycle.BuildLifecycleStopReminder(repo)
}

func BuildLifecyclePreCompactCapsule(repo string) LifecycleCompactResult {
	return lifecycle.BuildLifecyclePreCompactCapsule(repo)
}

func BuildLifecyclePostCompactReminder(repo string) LifecycleCompactResult {
	return lifecycle.BuildLifecyclePostCompactReminder(repo)
}

func ResolveProjectLifecycleState(repoRoot string) (ProjectLifecycleStatePlan, error) {
	return lifecycle.ResolveProjectLifecycleState(repoRoot)
}

func InitProjectLifecycleState(repoRoot string, confirm bool, metadata ...ProjectProfile) (ProjectLifecycleStatePlan, error) {
	return lifecycle.InitProjectLifecycleState(repoRoot, confirm, projectProfilesToLifecycle(metadata)...)
}

func ValidateProjectLifecycleState(repoRoot string) (ProjectLifecycleStatePlan, error) {
	return lifecycle.ValidateProjectLifecycleState(repoRoot)
}

func AppendDocUpkeepEvent(repoRoot string, event DocUpkeepEvent) (DocUpkeepAppendResult, error) {
	return lifecycle.AppendDocUpkeepEvent(repoRoot, event)
}

func ReadPendingDocUpkeepEvents(repoRoot string, limit int) ([]DocUpkeepEvent, ProjectLifecycleStatePlan, error) {
	return lifecycle.ReadPendingDocUpkeepEvents(repoRoot, limit)
}

func RecordStopNextActionRelay(repoRoot string, trigger NextActionJudgementTriggerResult) StopNextActionRelayResult {
	return lifecycle.RecordStopNextActionRelay(repoRoot, trigger)
}

func ClearStopNextActionRelay(repoRoot string) StopNextActionRelayResult {
	return lifecycle.ClearStopNextActionRelay(repoRoot)
}

func projectProfilesToLifecycle(profiles []ProjectProfile) []projectdocs.ProjectProfile {
	out := make([]projectdocs.ProjectProfile, 0, len(profiles))
	for _, profile := range profiles {
		out = append(out, profile)
	}
	return out
}

type NumberedNextActionsDecisionResult = nextaction.NumberedNextActionsDecisionResult
type NextActionCandidate = nextaction.NextActionCandidate
type NextActionJudgementTriggerResult = nextaction.NextActionJudgementTriggerResult
type NextActionAutoProceedResult = nextaction.NextActionAutoProceedResult
type NextActionAutoProceedLLMRequest = nextaction.NextActionAutoProceedLLMRequest

func BuildNumberedNextActionsDecision(message string, enforce bool, source string) NumberedNextActionsDecisionResult {
	return nextaction.BuildNumberedNextActionsDecision(message, enforce, source)
}

func BuildNextActionJudgementTrigger(message string) NextActionJudgementTriggerResult {
	return nextaction.BuildNextActionJudgementTrigger(message)
}

func BuildNextActionJudgementRelayReason(trigger NextActionJudgementTriggerResult) string {
	return nextaction.BuildJudgementRelayReason(trigger)
}

func EvaluateNextActionAutoProceed(message string, threshold float64) NextActionAutoProceedResult {
	return nextaction.EvaluateNextActionAutoProceed(message, threshold)
}

func EvaluateNextActionAutoProceedLLM(req NextActionAutoProceedLLMRequest, threshold float64) (NextActionAutoProceedResult, error) {
	return nextaction.EvaluateNextActionAutoProceedLLM(req, threshold)
}

func parseNextActionCandidates(message string) []NextActionCandidate {
	return nextaction.ParseCandidates(message)
}

func selectRecommendedNextAction(candidates []NextActionCandidate) *NextActionCandidate {
	return nextaction.SelectRecommendedCandidate(candidates)
}

func buildNextActionAutoProceedLLMPrompt(recommended NextActionCandidate, candidates []NextActionCandidate) string {
	return nextaction.BuildLLMPrompt(recommended, candidates)
}

const (
	WorkerStatusQueued    = coreworker.WorkerStatusQueued
	WorkerStatusRunning   = coreworker.WorkerStatusRunning
	WorkerStatusSucceeded = coreworker.WorkerStatusSucceeded
	WorkerStatusFailed    = coreworker.WorkerStatusFailed
	WorkerStatusCancelled = coreworker.WorkerStatusCancelled
)

type WorkerJob = coreworker.WorkerJob
type WorkerListResult = coreworker.WorkerListResult

func EnqueueWorkerJob(kind, payload string) (WorkerJob, error) {
	return coreworker.EnqueueWorkerJob(kind, payload)
}

func CancelWorkerJob(id string) (WorkerJob, error) {
	return coreworker.CancelWorkerJob(id)
}

func ReadWorkerJob(id string) (WorkerJob, error) {
	return coreworker.ReadWorkerJob(id)
}

func ListWorkerJobs() (WorkerListResult, error) {
	return coreworker.ListWorkerJobs()
}

func RunReadOnlyWorkerJob(kind, payload string, req CommandPolicyRequest) (WorkerJob, error) {
	return coreworker.RunReadOnlyWorkerJob(kind, payload, policy.CommandPolicyRequest(req))
}
