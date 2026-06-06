package main

import (
	"fmt"
	"os"

	"agent-harness/internal/core"
)

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
