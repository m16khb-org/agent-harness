package stateio

import (
	statestore "agent-harness/internal/adapter/outbound/state"
	"fmt"
	"strings"
)

func PromoteSelfAugmentBaseline(fromKey, baselineKey string, confirm, allowFailedSource bool) (SelfAugmentPromoteResult, error) {
	result := SelfAugmentPromoteResult{
		OK:          false,
		StateDir:    statestore.StateDir(),
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
	// Gate semantics (measured by the SV-B case): a baseline is the reference
	// every future compare regresses against, so promoting a failed run
	// silently poisons all later judgements. Refuse unless explicitly
	// overridden; dry-run stays diagnostic and just reports the flag.
	result.SourcePassed = snapshot.OK && snapshot.Summary.TerminationEligible
	if !confirm {
		result.OK = true
		return result, nil
	}
	if !result.SourcePassed && !allowFailedSource {
		return result, fmt.Errorf(
			"refusing to promote: source snapshot %q did not pass the gate (ok=%v, termination_eligible=%v); rerun self-verify or pass --allow-failed-source",
			fromKey, snapshot.OK, snapshot.Summary.TerminationEligible)
	}
	if err := WriteSelfAugmentSnapshotRecord(statestore.StateDir(), baselineKey, snapshot); err != nil {
		return result, err
	}
	state, err := statestore.StateRead(baselineKey)
	if err != nil {
		return result, err
	}
	result.OK = true
	result.Promoted = true
	result.Path = state.Path
	result.Bytes = state.Record.Bytes
	return result, nil
}
