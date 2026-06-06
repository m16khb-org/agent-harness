package selfworkflow

import (
	"fmt"
	"strings"

	"agent-harness/internal/core"
)

func PromoteSelfAugmentBaseline(fromKey, baselineKey string, confirm bool) (SelfAugmentPromoteResult, error) {
	result := SelfAugmentPromoteResult{
		OK:          false,
		StateDir:    core.StateDir(),
		FromKey:     fromKey,
		BaselineKey: baselineKey,
		Confirm:     confirm,
		DryRun:      !confirm,
	}
	if strings.TrimSpace(fromKey) == "" {
		return result, fmt.Errorf("from-key is required")
	}
	if strings.TrimSpace(baselineKey) == "" {
		return result, fmt.Errorf("baseline-key is required")
	}
	snapshot, err := ReadSelfAugmentStateSnapshot(fromKey)
	if err != nil {
		return result, fmt.Errorf("read source summary: %w", err)
	}
	result.SnapshotGeneratedAt = snapshot.GeneratedAt
	result.Summary = snapshot.Summary
	if !confirm {
		result.OK = true
		return result, nil
	}
	if err := WriteSelfAugmentSnapshotRecord(core.StateDir(), baselineKey, snapshot); err != nil {
		return result, err
	}
	state, err := core.StateRead(baselineKey)
	if err != nil {
		return result, err
	}
	result.OK = true
	result.Promoted = true
	result.Path = state.Path
	result.Bytes = state.Record.Bytes
	return result, nil
}
