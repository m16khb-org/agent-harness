package core

import (
	"agent-harness/internal/core/lifecycle"
	"agent-harness/internal/core/projectdocs"
)

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
