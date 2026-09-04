package issueopsrouting

import (
	"fmt"
	"strings"
	"time"

	issueopsroutingcontract "issueops/internal/contract/issueopsrouting"
)

const MaxTraceEntries = 500

func NewEntry(phase, skill string, at time.Time) (issueopsroutingcontract.Entry, error) {
	phase = strings.TrimSpace(phase)
	skill = strings.TrimSpace(skill)
	if phase == "" {
		return issueopsroutingcontract.Entry{}, fmt.Errorf("routing phase is required")
	}
	if skill == "" {
		return issueopsroutingcontract.Entry{}, fmt.Errorf("routing skill is required")
	}
	if len(phase) > 64 || len(skill) > 64 {
		return issueopsroutingcontract.Entry{}, fmt.Errorf(
			"routing phase and skill must not exceed 64 bytes each",
		)
	}
	return issueopsroutingcontract.Entry{
		Phase: phase,
		Skill: skill,
		At:    at.UTC().Format(time.RFC3339Nano),
	}, nil
}

func Append(
	record issueopsroutingcontract.Record,
	entry issueopsroutingcontract.Entry,
) (issueopsroutingcontract.Record, bool, error) {
	for _, observed := range record.RoutingTrace {
		if strings.EqualFold(observed.Phase, entry.Phase) &&
			strings.EqualFold(observed.Skill, entry.Skill) {
			return record, false, nil
		}
	}
	if len(record.RoutingTrace) >= MaxTraceEntries {
		return issueopsroutingcontract.Record{OK: false}, false, fmt.Errorf(
			"routing trace is full (%d entries)",
			MaxTraceEntries,
		)
	}
	record.RoutingTrace = append(record.RoutingTrace, entry)
	record.UpdatedAt = entry.At
	record.OK = true
	return record, true, nil
}

func ScoreRecord(
	record issueopsroutingcontract.Record,
	expected []issueopsroutingcontract.Expected,
) issueopsroutingcontract.Result {
	missing := make([]issueopsroutingcontract.Expected, 0)
	for _, expectedEntry := range expected {
		if !containsRecordPairing(record.RoutingTrace, expectedEntry) {
			missing = append(missing, expectedEntry)
		}
	}
	return issueopsroutingcontract.Result{OK: len(missing) == 0, Missing: missing}
}

func Score(
	expected []issueopsroutingcontract.Expected,
	observed []issueopsroutingcontract.Expected,
) issueopsroutingcontract.Result {
	missing := make([]issueopsroutingcontract.Expected, 0)
	for _, expectedEntry := range expected {
		if !containsPairing(observed, expectedEntry) {
			missing = append(missing, expectedEntry)
		}
	}
	return issueopsroutingcontract.Result{OK: len(missing) == 0, Missing: missing}
}

func containsPairing(
	trace []issueopsroutingcontract.Expected,
	expected issueopsroutingcontract.Expected,
) bool {
	for _, entry := range trace {
		if strings.EqualFold(strings.TrimSpace(entry.Phase), strings.TrimSpace(expected.Phase)) &&
			strings.EqualFold(strings.TrimSpace(entry.Skill), strings.TrimSpace(expected.Skill)) {
			return true
		}
	}
	return false
}

func containsRecordPairing(
	trace []issueopsroutingcontract.Entry,
	expected issueopsroutingcontract.Expected,
) bool {
	for _, entry := range trace {
		if strings.EqualFold(strings.TrimSpace(entry.Phase), strings.TrimSpace(expected.Phase)) &&
			strings.EqualFold(strings.TrimSpace(entry.Skill), strings.TrimSpace(expected.Skill)) {
			return true
		}
	}
	return false
}
