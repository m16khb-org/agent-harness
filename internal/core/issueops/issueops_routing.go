package issueops

import (
	"fmt"
	"strings"
	"time"

	"context"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/issueops/benchmark"
)

// RoutingFidelityResult reports whether a cycle's live routing covered every
// expected skill-at-phase pairing.
type RoutingFidelityResult = benchmark.RoutingFidelityResult

// ScoreLiveRoutingFidelity scores a cycle's recorded live routing trace against
// the expected (phase, skill) pairings using the same logic as the benchmark
// skill_routing_fidelity dimension — but on observed activation rather than a
// synthesized trace, so the score reflects what really happened in the run.
func ScoreLiveRoutingFidelity(record issueops.IssueOpsRecord, expected []SkillRouting) RoutingFidelityResult {
	return benchmark.RoutingFidelity(expected, RoutingTraceAsSkillRouting(record))
}

// maxRoutingTraceEntries bounds the live routing trace so a long-running cycle
// cannot grow the record without limit.
const maxRoutingTraceEntries = 500

// RecordIssueOpsRouting appends a (phase, skill) pairing to the cycle's live
// RoutingTrace, capturing which pioneer/CS skill actually fired at which phase
// during a real run. Unlike the benchmark fixture's synthesized trace, this is
// observed activation, so skill_routing_fidelity can be scored against real
// behavior rather than a tautology. It is idempotent per (phase, skill).
func RecordIssueOpsRouting(stateRoot, id, phase, skill string) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsRouting(stateRoot, id, phase, skill, nil)
}

func RecordIssueOpsRoutingWithActor(stateRoot, id, phase, skill string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsRouting(stateRoot, id, phase, skill, &actor)
}

func recordIssueOpsRouting(stateRoot, id, phase, skill string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = recordIssueOpsRoutingLocked(stateRoot, id, phase, skill)
		return e
	})
	return rec, err
}

func recordIssueOpsRoutingLocked(stateRoot, id, phase, skill string) (issueops.IssueOpsRecord, error) {
	phase = strings.TrimSpace(phase)
	skill = strings.TrimSpace(skill)
	if phase == "" {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("routing phase is required")
	}
	if skill == "" {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("routing skill is required")
	}
	if len(phase) > 64 || len(skill) > 64 {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("routing phase and skill must not exceed 64 bytes each")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	for _, e := range record.RoutingTrace {
		if strings.EqualFold(e.Phase, phase) && strings.EqualFold(e.Skill, skill) {
			// Already recorded; report the unchanged record without a rewrite.
			return record, nil
		}
	}
	if len(record.RoutingTrace) >= maxRoutingTraceEntries {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("routing trace is full (%d entries)", maxRoutingTraceEntries)
	}
	record.RoutingTrace = append(record.RoutingTrace, issueops.SkillRoutingEntry{
		Phase: phase,
		Skill: skill,
		At:    time.Now().UTC().Format(time.RFC3339Nano),
	})
	return touchAndWriteIssueOps(stateRoot, record)
}

// RoutingTraceAsSkillRouting projects a cycle's live RoutingTrace onto the
// benchmark SkillRouting shape so skill_routing_fidelity can score a real run's
// trace instead of the synthesized one.
func RoutingTraceAsSkillRouting(record issueops.IssueOpsRecord) []SkillRouting {
	out := make([]SkillRouting, 0, len(record.RoutingTrace))
	for _, e := range record.RoutingTrace {
		out = append(out, SkillRouting{Phase: e.Phase, Skill: e.Skill})
	}
	return out
}
