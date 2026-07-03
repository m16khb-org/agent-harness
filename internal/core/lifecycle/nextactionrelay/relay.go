package nextactionrelay

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/lifecycle/model"
	"agent-harness/internal/core/nextaction"
)

type Store struct {
	Validate  func(repoRoot string) (model.ProjectLifecycleStatePlan, error)
	Init      func(repoRoot string, confirm bool) (model.ProjectLifecycleStatePlan, error)
	WriteJSON func(path string, value any, perm os.FileMode) error
}

func Record(store Store, repoRoot string, trigger nextaction.NextActionJudgementTriggerResult) model.StopNextActionRelayResult {
	fingerprint := fingerprint(trigger)
	result := model.StopNextActionRelayResult{OK: true, Fingerprint: fingerprint}
	if strings.TrimSpace(fingerprint) == "" {
		result.Reason = "no_next_action_fingerprint"
		return result
	}
	plan, err := store.Validate(repoRoot)
	if err != nil {
		result.Warnings = append(result.Warnings, "project_lifecycle_state_error")
		return result
	}
	if !plan.Exists {
		plan, err = store.Init(repoRoot, true)
		if err != nil {
			result.Warnings = append(result.Warnings, "project_lifecycle_state_init_error")
			return result
		}
	}
	if !plan.NamespaceValid {
		result.Warnings = append(result.Warnings, "project_lifecycle_namespace_mismatch")
		return result
	}
	path := filepath.Join(plan.ProjectStateDir, model.StopNextActionRelayFile)
	result.Path = path
	var previous model.StopNextActionRelayRecord
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &previous); err == nil && previous.SchemaVersion == model.ProjectLifecycleSchemaVersion && previous.Fingerprint != "" {
			result.ShouldRelay = false
			if previous.Fingerprint == fingerprint {
				result.Reason = "duplicate_next_action_relay"
			} else {
				result.Reason = "pending_next_action_relay"
			}
			return result
		}
	}
	candidates := make([]model.StopNextActionRelayCandidate, 0, len(trigger.Candidates))
	for _, candidate := range trigger.Candidates {
		candidates = append(candidates, model.StopNextActionRelayCandidate{
			Index:       candidate.Index,
			Recommended: candidate.Recommended,
			Text:        strings.TrimSpace(candidate.Text),
		})
	}
	record := model.StopNextActionRelayRecord{
		SchemaVersion:    model.ProjectLifecycleSchemaVersion,
		Fingerprint:      fingerprint,
		RecommendedIndex: trigger.RecommendedIndex,
		RecommendedText:  trigger.RecommendedText,
		Candidates:       candidates,
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := store.WriteJSON(path, record, 0o600); err != nil {
		result.Warnings = append(result.Warnings, "stop_next_action_relay_write_error")
		return result
	}
	result.ShouldRelay = true
	result.Reason = "recorded_next_action_relay"
	return result
}

// Read returns the pending relay record so the next user prompt can expand a
// bare choice reply ("1", "2번") back into the chosen option's full text. It
// never mutates state; Clear still owns record removal.
func Read(store Store, repoRoot string) (model.StopNextActionRelayRecord, bool) {
	var record model.StopNextActionRelayRecord
	plan, err := store.Validate(repoRoot)
	if err != nil || !plan.Exists || !plan.NamespaceValid {
		return record, false
	}
	b, err := os.ReadFile(filepath.Join(plan.ProjectStateDir, model.StopNextActionRelayFile))
	if err != nil {
		return record, false
	}
	if err := json.Unmarshal(b, &record); err != nil ||
		record.SchemaVersion != model.ProjectLifecycleSchemaVersion ||
		strings.TrimSpace(record.Fingerprint) == "" {
		return model.StopNextActionRelayRecord{}, false
	}
	return record, true
}

func Clear(store Store, repoRoot string) model.StopNextActionRelayResult {
	result := model.StopNextActionRelayResult{OK: true}
	plan, err := store.Validate(repoRoot)
	if err != nil || !plan.Exists || !plan.NamespaceValid {
		return result
	}
	path := filepath.Join(plan.ProjectStateDir, model.StopNextActionRelayFile)
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

func fingerprint(trigger nextaction.NextActionJudgementTriggerResult) string {
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
