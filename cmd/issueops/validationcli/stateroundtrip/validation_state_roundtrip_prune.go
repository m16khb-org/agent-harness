package stateroundtrip

import (
	"encoding/json"
	statecontract "issueops/internal/contract/state"
)

func (s *stateRoundtripStateSession) validatePrune() StepResult {
	oldKey := s.input.key + "-old"
	oldWrite := s.run("state old write", s.input.binary, "state", "write", "--key", oldKey, "--value", "old state", "--json")
	if !oldWrite.OK {
		return s.combineFailed(oldWrite)
	}
	var oldWriteResult statecontract.StateResult
	if err := json.Unmarshal([]byte(oldWrite.Stdout), &oldWriteResult); err != nil {
		return s.fail(err.Error())
	}
	oldWriteResult.Record.UpdatedAt = "2000-01-01T00:00:00Z"
	if _, err := WriteStateRecord(s.input.tempState, oldKey, oldWriteResult.Record); err != nil {
		return s.fail(err.Error())
	}

	pruneDry := s.run("state prune dry-run", s.input.binary, "state", "prune", "--max-age", "1h", "--json")
	if !pruneDry.OK {
		return s.combineFailed(pruneDry)
	}
	var pruneDryResult statecontract.StatePruneResult
	if err := json.Unmarshal([]byte(pruneDry.Stdout), &pruneDryResult); err != nil {
		return s.fail(err.Error())
	}
	if !pruneDryResult.OK || !pruneDryResult.DryRun || !containsString(pruneDryResult.DeletedKeys, oldKey) || !containsString(pruneDryResult.KeptKeys, s.input.key) {
		return s.fail("state prune dry-run did not classify old/fresh keys")
	}

	pruneConfirm := s.run("state prune confirm", s.input.binary, "state", "prune", "--max-age", "1h", "--confirm", "--json")
	if !pruneConfirm.OK {
		return s.combineFailed(pruneConfirm)
	}
	var pruneConfirmResult statecontract.StatePruneResult
	if err := json.Unmarshal([]byte(pruneConfirm.Stdout), &pruneConfirmResult); err != nil {
		return s.fail(err.Error())
	}
	if !pruneConfirmResult.OK || pruneConfirmResult.DryRun || !pruneConfirmResult.Confirm || !containsString(pruneConfirmResult.DeletedKeys, oldKey) {
		return s.fail("state prune confirm did not delete old key")
	}

	listAfterPrune := s.run("state list after prune", s.input.binary, "state", "list", "--json")
	if !listAfterPrune.OK {
		return s.combineFailed(listAfterPrune)
	}
	var listAfterPruneResult statecontract.StateListResult
	if err := json.Unmarshal([]byte(listAfterPrune.Stdout), &listAfterPruneResult); err != nil {
		return s.fail(err.Error())
	}
	if !containsString(listAfterPruneResult.Keys, s.input.key) || containsString(listAfterPruneResult.Keys, oldKey) {
		return s.fail("state prune did not preserve fresh key and remove old key")
	}
	return StepResult{OK: true}
}
