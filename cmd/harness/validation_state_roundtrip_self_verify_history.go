package main

import (
	"encoding/json"
	"path/filepath"

	"agent-harness/internal/core"
)

func (s *stateRoundtripSelfVerifySession) validateHistoryAndRetention(baselineCompareKey, candidateCompareKey, promotedBaselineKey string) StepResult {
	input := s.input
	key := input.key
	history := s.run("self verify history", input.binary, "self-verify", "history", "--prefix", key+"-", "--json")
	if !history.OK {
		return s.combineFailed(history)
	}
	var historyResult SelfAugmentHistoryResult
	if err := json.Unmarshal([]byte(history.Stdout), &historyResult); err != nil {
		return s.fail(err.Error())
	}
	historyKeys := []string{}
	for _, entry := range historyResult.Entries {
		historyKeys = append(historyKeys, entry.Key)
	}
	if !historyResult.OK || historyResult.TotalMatches < 3 || !containsString(historyKeys, baselineCompareKey) || !containsString(historyKeys, candidateCompareKey) || !containsString(historyKeys, promotedBaselineKey) {
		return s.fail("self-verify history did not list saved baseline/candidate/promoted summaries")
	}

	retentionDry := s.run("self verify history retention dry-run", input.binary, "self-verify", "history", "--prefix", key+"-", "--retention-limit", "1", "--prune-retention", "--json")
	if !retentionDry.OK {
		return s.combineFailed(retentionDry)
	}
	var retentionDryResult SelfAugmentHistoryResult
	if err := json.Unmarshal([]byte(retentionDry.Stdout), &retentionDryResult); err != nil {
		return s.fail(err.Error())
	}
	if retentionDryResult.Retention == nil || !retentionDryResult.Retention.DryRun || retentionDryResult.Retention.Confirm || retentionDryResult.Retention.Limit != 1 || len(retentionDryResult.Retention.CandidateKeys) == 0 || len(retentionDryResult.Retention.DeletedKeys) != 0 {
		return s.fail("self-verify history retention dry-run did not classify prune candidates safely")
	}

	retentionConfirm := s.run("self verify history retention confirm", input.binary, "self-verify", "history", "--prefix", key+"-", "--retention-limit", "1", "--prune-retention", "--confirm", "--json")
	if !retentionConfirm.OK {
		return s.combineFailed(retentionConfirm)
	}
	var retentionConfirmResult SelfAugmentHistoryResult
	if err := json.Unmarshal([]byte(retentionConfirm.Stdout), &retentionConfirmResult); err != nil {
		return s.fail(err.Error())
	}
	if retentionConfirmResult.Retention == nil || retentionConfirmResult.Retention.DryRun || !retentionConfirmResult.Retention.Confirm || len(retentionConfirmResult.Retention.DeletedKeys) == 0 {
		return s.fail("self-verify history retention confirm did not delete prune candidates")
	}

	historyAfterRetention := s.run("self verify history after retention", input.binary, "self-verify", "history", "--prefix", key+"-", "--json")
	if !historyAfterRetention.OK {
		return s.combineFailed(historyAfterRetention)
	}
	var historyAfterRetentionResult SelfAugmentHistoryResult
	if err := json.Unmarshal([]byte(historyAfterRetention.Stdout), &historyAfterRetentionResult); err != nil {
		return s.fail(err.Error())
	}
	if historyAfterRetentionResult.TotalMatches > 1 {
		return s.fail("self-verify history retention confirm left too many matching summaries")
	}

	corruptPath := filepath.Join(input.tempState, "corrupt.json")
	if err := input.deps.writeFile(corruptPath, []byte("{not json\n"), 0o600); err != nil {
		return s.fail(err.Error())
	}
	doctor := s.run("state doctor", input.binary, "state", "doctor", "--json")
	if !doctor.OK {
		return s.combineFailed(doctor)
	}
	var doctorResult core.StateDoctorResult
	if err := json.Unmarshal([]byte(doctor.Stdout), &doctorResult); err != nil {
		return s.fail(err.Error())
	}
	if !doctorResult.OK || doctorResult.Healthy || !containsString(doctorResult.ValidKeys, key) || !stateDoctorHasIssueCode(doctorResult.Issues, "invalid_json") {
		return s.fail("state doctor did not report corrupt fixture and preserve valid key")
	}
	return StepResult{OK: true}
}
