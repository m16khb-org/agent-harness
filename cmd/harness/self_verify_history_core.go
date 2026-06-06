package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"agent-harness/internal/core"
)

func selfAugmentHistory(prefix string, limit int, retentionOptions ...selfAugmentHistoryRetentionOptions) (SelfAugmentHistoryResult, error) {
	result := SelfAugmentHistoryResult{
		OK:       false,
		StateDir: core.StateDir(),
		Prefix:   prefix,
		Limit:    limit,
		Entries:  []SelfAugmentHistoryEntry{},
		Skipped:  []SelfAugmentHistorySkipped{},
		Warnings: []string{},
	}
	if limit < 0 {
		return result, fmt.Errorf("limit must be non-negative")
	}
	retention := selfAugmentHistoryRetentionOptions{}
	if len(retentionOptions) > 0 {
		retention = retentionOptions[0]
	}
	if retention.Limit < 0 {
		return result, fmt.Errorf("retention-limit must be non-negative")
	}
	if retention.Confirm && !retention.PruneRequested {
		return result, fmt.Errorf("confirm requires --prune-retention")
	}
	if retention.PruneRequested && retention.Limit <= 0 {
		return result, fmt.Errorf("prune-retention requires a positive --retention-limit")
	}
	list, err := core.StateList()
	if err != nil {
		return result, err
	}
	for _, record := range list.Records {
		if prefix != "" && !strings.HasPrefix(record.Key, prefix) {
			continue
		}
		state, err := core.StateRead(record.Key)
		if err != nil {
			result.Skipped = append(result.Skipped, SelfAugmentHistorySkipped{Key: record.Key, Reason: "state_read:" + err.Error()})
			continue
		}
		var snapshot SelfAugmentStateSnapshot
		if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
			result.Skipped = append(result.Skipped, SelfAugmentHistorySkipped{Key: record.Key, Reason: "not_json_summary"})
			continue
		}
		if !isSelfVerificationSummaryKind(snapshot.Kind) {
			result.Skipped = append(result.Skipped, SelfAugmentHistorySkipped{Key: record.Key, Reason: "kind:" + snapshot.Kind})
			continue
		}
		if snapshot.SchemaVersion != 1 {
			result.Skipped = append(result.Skipped, SelfAugmentHistorySkipped{Key: record.Key, Reason: fmt.Sprintf("schema:%d", snapshot.SchemaVersion)})
			continue
		}
		if _, ok := parseSelfAugmentTimestamp(snapshot.GeneratedAt); !ok {
			result.Warnings = append(result.Warnings, "invalid_generated_at:"+record.Key)
		}
		result.Entries = append(result.Entries, SelfAugmentHistoryEntry{
			Key:          record.Key,
			UpdatedAt:    record.UpdatedAt,
			Bytes:        record.Bytes,
			GeneratedAt:  snapshot.GeneratedAt,
			OK:           snapshot.OK,
			Iterations:   snapshot.Iterations,
			BaseSeed:     snapshot.BaseSeed,
			ElapsedMS:    snapshot.ElapsedMS,
			TotalRuns:    snapshot.Summary.TotalRuns,
			TotalSteps:   snapshot.Summary.TotalSteps,
			FailedSteps:  snapshot.Summary.FailedSteps,
			StepLabels:   nonNilStringSlice(snapshot.Summary.StepLabels),
			SlowestSteps: nonNilSlowStepSlice(snapshot.Summary.SlowestSteps),
		})
	}
	sort.Slice(result.Entries, func(i, j int) bool {
		left, leftOK := parseSelfAugmentTimestamp(result.Entries[i].GeneratedAt)
		right, rightOK := parseSelfAugmentTimestamp(result.Entries[j].GeneratedAt)
		if leftOK != rightOK {
			return leftOK
		}
		if leftOK && !left.Equal(right) {
			return left.After(right)
		}
		leftUpdated, leftUpdatedOK := parseSelfAugmentTimestamp(result.Entries[i].UpdatedAt)
		rightUpdated, rightUpdatedOK := parseSelfAugmentTimestamp(result.Entries[j].UpdatedAt)
		if leftUpdatedOK != rightUpdatedOK {
			return leftUpdatedOK
		}
		if leftUpdatedOK && !leftUpdated.Equal(rightUpdated) {
			return leftUpdated.After(rightUpdated)
		}
		return result.Entries[i].Key < result.Entries[j].Key
	})
	sort.Slice(result.Skipped, func(i, j int) bool { return result.Skipped[i].Key < result.Skipped[j].Key })
	sort.Strings(result.Warnings)
	result.TotalMatches = len(result.Entries)
	if retention.Limit > 0 {
		if err := applySelfAugmentHistoryRetention(&result, retention); err != nil {
			return result, err
		}
		sort.Strings(result.Warnings)
	}
	if limit > 0 && len(result.Entries) > limit {
		result.Entries = result.Entries[:limit]
	}
	result.Returned = len(result.Entries)
	result.OK = true
	return result, nil
}

func applySelfAugmentHistoryRetention(result *SelfAugmentHistoryResult, options selfAugmentHistoryRetentionOptions) error {
	retention := &SelfAugmentHistoryRetention{
		Enabled:        true,
		Limit:          options.Limit,
		TotalMatches:   result.TotalMatches,
		RetainedKeys:   []string{},
		CandidateKeys:  []string{},
		DeletedKeys:    []string{},
		PruneRequested: options.PruneRequested,
		Confirm:        options.Confirm,
		DryRun:         options.PruneRequested && !options.Confirm,
		Recommendation: "within_retention_budget",
	}
	for i, entry := range result.Entries {
		if i < options.Limit {
			retention.RetainedKeys = append(retention.RetainedKeys, entry.Key)
			continue
		}
		retention.CandidateKeys = append(retention.CandidateKeys, entry.Key)
	}
	if len(retention.CandidateKeys) > 0 {
		retention.Recommendation = fmt.Sprintf("prune %d history checkpoint(s) beyond retention-limit=%d after reviewing dry-run output", len(retention.CandidateKeys), options.Limit)
		result.Warnings = append(result.Warnings, fmt.Sprintf("history_retention_candidates:%d", len(retention.CandidateKeys)))
	}
	if options.PruneRequested && options.Confirm {
		for _, key := range retention.CandidateKeys {
			state, err := core.StateRead(key)
			if err != nil {
				return fmt.Errorf("read retention candidate %q: %w", key, err)
			}
			if err := os.Remove(state.Path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("delete retention candidate %q: %w", key, err)
			}
			retention.DeletedKeys = append(retention.DeletedKeys, key)
		}
		retention.Recommendation = fmt.Sprintf("deleted %d history checkpoint(s) beyond retention-limit=%d", len(retention.DeletedKeys), options.Limit)
	}
	result.Retention = retention
	return nil
}

func parseSelfAugmentTimestamp(value string) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, true
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func nonNilStringSlice(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func nonNilSlowStepSlice(items []SelfAugmentSlowStep) []SelfAugmentSlowStep {
	if items == nil {
		return []SelfAugmentSlowStep{}
	}
	return items
}
