package issueops

import (
	"fmt"
	"strings"
)

func ValidateRecord(record IssueOpsRecord) error {
	if record.SchemaVersion != IssueOpsSchemaVersion {
		return fmt.Errorf("issueops record schema version is invalid")
	}
	if !knownRecordPhase(record.Phase) {
		return fmt.Errorf("issueops record phase %q is invalid", record.Phase)
	}
	for phase, entry := range record.PhaseLedger {
		if !knownRecordPhase(phase) || entry.Phase != phase {
			return fmt.Errorf("issueops phase ledger identity is invalid")
		}
	}
	if record.PlanPrep != nil {
		items := []struct {
			name string
			item IssueOpsPlanPrepItem
		}{
			{name: "prior_decisions", item: record.PlanPrep.PriorDecisions},
			{name: "related_issues", item: record.PlanPrep.RelatedIssues},
			{name: "web_research", item: record.PlanPrep.WebResearch},
			{name: "codebase_survey", item: record.PlanPrep.CodebaseSurvey},
		}
		for _, item := range items {
			if err := validateRecordPlanPrepItem(item.name, item.item); err != nil {
				return err
			}
		}
	}
	if record.Intent != nil {
		if _, err := NormalizeIntentClass(record.Intent.IntentClass); err != nil {
			return err
		}
	}
	for _, feedback := range record.Feedback {
		if !KnownFeedbackClassification(feedback.Classification) {
			return fmt.Errorf("issueops feedback classification is invalid")
		}
		if !KnownFeedbackResolution(feedback.Resolution) {
			return fmt.Errorf("issueops feedback resolution is invalid")
		}
	}
	for _, event := range record.RegressEvents {
		if !knownRecordPhase(event.FromPhase) {
			return fmt.Errorf("issueops regress event phase is invalid")
		}
	}
	for _, routing := range record.RoutingTrace {
		if !knownRecordPhase(IssueOpsPhase(routing.Phase)) {
			return fmt.Errorf("issueops routing phase is invalid")
		}
	}
	if record.BranchPrepare != nil &&
		record.BranchPrepare.Provider != "github" &&
		record.BranchPrepare.Provider != "gitlab" {
		return fmt.Errorf("issueops branch provider is invalid")
	}
	if record.RemoteArtifact != nil &&
		!knownRemoteArtifactIdentity(record.RemoteArtifact.Provider, record.RemoteArtifact.Kind) {
		return fmt.Errorf("issueops remote artifact identity is invalid")
	}
	for _, child := range record.ChildCycles {
		if !knownChildValidationVerdict(child.ValidationVerdict) {
			return fmt.Errorf("issueops child validation verdict is invalid")
		}
	}
	if record.CleanupFinishFailure != nil &&
		!knownCleanupFinishFailureStep(record.CleanupFinishFailure.Step) {
		return fmt.Errorf("issueops cleanup finish failure step is invalid")
	}
	if record.CleanupAbandonFailure != nil &&
		!knownCleanupAbandonFailureStep(record.CleanupAbandonFailure.Step) {
		return fmt.Errorf("issueops cleanup abandon failure step is invalid")
	}
	if record.IssueCreateIntent != nil {
		if err := ValidateIssueCreateIntent(*record.IssueCreateIntent); err != nil {
			return err
		}
		if record.IssueCreateIntent.Status == IssueCreateIntentCompleted &&
			strings.TrimSpace(record.IssueURL) != strings.TrimSpace(record.IssueCreateIntent.CanonicalURL) {
			return fmt.Errorf("completed issue create intent canonical_url must match issue_url")
		}
	}
	if record.Execution != nil {
		if err := ValidateExecution(*record.Execution); err != nil {
			return err
		}
	}
	return nil
}

func KnownFeedbackClassification(classification string) bool {
	switch classification {
	case "",
		"contract_change",
		"defect",
		"question",
		"noise",
		"valid_review",
		"stale_review",
		"rollout_evidence_missing",
		"environment_debt":
		return true
	default:
		return false
	}
}

func KnownFeedbackResolution(resolution string) bool {
	switch resolution {
	case "", "valid-defect", "question-answered", "noise-dismissed":
		return true
	default:
		return false
	}
}

func knownRemoteArtifactIdentity(provider, kind string) bool {
	return provider == "github" && kind == "pr" ||
		provider == "gitlab" && kind == "mr"
}

func knownChildValidationVerdict(verdict string) bool {
	switch verdict {
	case "", "accepted", "rejected", "dropped":
		return true
	default:
		return false
	}
}

func knownCleanupFinishFailureStep(step string) bool {
	switch step {
	case CleanupFailureStepWorkspaceProcessesStop,
		CleanupFailureStepOrcaRemove,
		CleanupFailureStepWorktreeRemove,
		CleanupFailureStepBranchDelete,
		CleanupFailureStepRecordDelete:
		return true
	default:
		return false
	}
}

func knownCleanupAbandonFailureStep(step string) bool {
	switch step {
	case CleanupFailureStepApplying,
		CleanupFailureStepWorkspaceProcessesStop,
		CleanupFailureStepWorktreeRemove,
		CleanupFailureStepBranchDelete,
		CleanupFailureStepRecordDelete:
		return true
	default:
		return false
	}
}

func knownRecordPhase(phase IssueOpsPhase) bool {
	for _, known := range IssueOpsPhases {
		if phase == known {
			return true
		}
	}
	return false
}

func validateRecordPlanPrepItem(name string, item IssueOpsPlanPrepItem) error {
	switch item.Status {
	case "evidence":
		if len(item.Evidence) == 0 || strings.TrimSpace(item.WaiveReason) != "" {
			return fmt.Errorf("issueops plan_prep %s evidence is invalid", name)
		}
	case "waived":
		if len(item.Evidence) != 0 || strings.TrimSpace(item.WaiveReason) == "" {
			return fmt.Errorf("issueops plan_prep %s waiver is invalid", name)
		}
	default:
		return fmt.Errorf("issueops plan_prep %s status %q is invalid", name, item.Status)
	}
	return nil
}
