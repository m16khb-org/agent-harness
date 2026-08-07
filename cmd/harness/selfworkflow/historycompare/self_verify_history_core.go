package historycompare

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"agent-harness/cmd/harness/selfworkflow/stateio"
	"agent-harness/internal/adapter/core"
)

func SelfAugmentHistory(prefix string, limit int, retentionOptions ...SelfAugmentHistoryRetentionOptions) (SelfAugmentHistoryResult, error) {
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
	retention := SelfAugmentHistoryRetentionOptions{}
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
		if !stateio.IsSelfVerificationSummaryKind(snapshot.Kind) {
			result.Skipped = append(result.Skipped, SelfAugmentHistorySkipped{Key: record.Key, Reason: "kind:" + snapshot.Kind})
			continue
		}
		if snapshot.SchemaVersion != 1 {
			result.Skipped = append(result.Skipped, SelfAugmentHistorySkipped{Key: record.Key, Reason: fmt.Sprintf("schema:%d", snapshot.SchemaVersion)})
			continue
		}
		if _, ok := ParseSelfAugmentTimestamp(snapshot.GeneratedAt); !ok {
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
			StepLabels:   NonNilStringSlice(snapshot.Summary.StepLabels),
			SlowestSteps: NonNilSlowStepSlice(snapshot.Summary.SlowestSteps),
		})
	}
	sort.Slice(result.Entries, func(i, j int) bool {
		left, leftOK := ParseSelfAugmentTimestamp(result.Entries[i].GeneratedAt)
		right, rightOK := ParseSelfAugmentTimestamp(result.Entries[j].GeneratedAt)
		if leftOK != rightOK {
			return leftOK
		}
		if leftOK && !left.Equal(right) {
			return left.After(right)
		}
		leftUpdated, leftUpdatedOK := ParseSelfAugmentTimestamp(result.Entries[i].UpdatedAt)
		rightUpdated, rightUpdatedOK := ParseSelfAugmentTimestamp(result.Entries[j].UpdatedAt)
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
		if err := ApplySelfAugmentHistoryRetention(&result, retention); err != nil {
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
