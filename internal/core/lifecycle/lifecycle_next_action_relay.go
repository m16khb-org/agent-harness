package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func RecordStopNextActionRelay(repoRoot string, trigger NextActionJudgementTriggerResult) StopNextActionRelayResult {
	fingerprint := stopNextActionRelayFingerprint(trigger)
	result := StopNextActionRelayResult{OK: true, Fingerprint: fingerprint}
	if strings.TrimSpace(fingerprint) == "" {
		result.Reason = "no_next_action_fingerprint"
		return result
	}
	plan, err := ValidateProjectLifecycleState(repoRoot)
	if err != nil {
		result.Warnings = append(result.Warnings, "project_lifecycle_state_error")
		return result
	}
	if !plan.Exists {
		plan, err = InitProjectLifecycleState(repoRoot, true)
		if err != nil {
			result.Warnings = append(result.Warnings, "project_lifecycle_state_init_error")
			return result
		}
	}
	if !plan.NamespaceValid {
		result.Warnings = append(result.Warnings, "project_lifecycle_namespace_mismatch")
		return result
	}
	path := filepath.Join(plan.ProjectStateDir, stopNextActionRelayFile)
	result.Path = path
	var previous StopNextActionRelayRecord
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &previous); err == nil && previous.SchemaVersion == ProjectLifecycleSchemaVersion && previous.Fingerprint != "" {
			result.ShouldRelay = false
			if previous.Fingerprint == fingerprint {
				result.Reason = "duplicate_next_action_relay"
			} else {
				result.Reason = "pending_next_action_relay"
			}
			return result
		}
	}
	record := StopNextActionRelayRecord{
		SchemaVersion:    ProjectLifecycleSchemaVersion,
		Fingerprint:      fingerprint,
		RecommendedIndex: trigger.RecommendedIndex,
		RecommendedText:  trigger.RecommendedText,
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := writeJSONAtomic(path, record, 0o600); err != nil {
		result.Warnings = append(result.Warnings, "stop_next_action_relay_write_error")
		return result
	}
	result.ShouldRelay = true
	result.Reason = "recorded_next_action_relay"
	return result
}

func ClearStopNextActionRelay(repoRoot string) StopNextActionRelayResult {
	result := StopNextActionRelayResult{OK: true}
	plan, err := ValidateProjectLifecycleState(repoRoot)
	if err != nil || !plan.Exists || !plan.NamespaceValid {
		return result
	}
	path := filepath.Join(plan.ProjectStateDir, stopNextActionRelayFile)
	result.Path = path
	if err := os.Remove(path); err == nil {
		result.Reason = "cleared_next_action_relay"
	} else if os.IsNotExist(err) {
		result.Reason = "no_next_action_relay"
	} else {
		result.Warnings = append(result.Warnings, "stop_next_action_relay_clear_error")
	}
	return result
}

func stopNextActionRelayFingerprint(trigger NextActionJudgementTriggerResult) string {
	if len(trigger.Candidates) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("stop-next-action-relay:v1\n")
	for _, candidate := range trigger.Candidates {
		b.WriteString(fmt.Sprintf("%d|%t|%s\n", candidate.Index, candidate.Recommended, strings.TrimSpace(candidate.Text)))
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
