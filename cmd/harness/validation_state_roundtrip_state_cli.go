package main

import (
	"encoding/json"
	"path/filepath"
	"time"

	"agent-harness/internal/core"
)

type validateStateRoundtripStateInput struct {
	binary    string
	root      string
	tempState string
	key       string
	content   string
	env       []string
	started   time.Time
	deps      stateRoundtripValidationDeps
}

type validateStateRoundtripStateResult struct {
	step        StepResult
	stdoutParts []string
	commands    []string
}

type stateRoundtripStateSession struct {
	input       validateStateRoundtripStateInput
	stdoutParts []string
	commands    []string
}

func validateStateRoundtripStateCLI(input validateStateRoundtripStateInput) validateStateRoundtripStateResult {
	session := stateRoundtripStateSession{input: input}
	if step := session.validateWriteReadList(); !step.OK {
		return session.result(step)
	}
	if step := session.validatePrune(); !step.OK {
		return session.result(step)
	}
	if step := session.validateMigrateAndDoctor(); !step.OK {
		return session.result(step)
	}
	return session.result(StepResult{OK: true})
}

func (s *stateRoundtripStateSession) run(label string, command ...string) StepResult {
	step := s.input.deps.run(s.input.root, label, 30*time.Second, "", s.input.env, command...)
	s.stdoutParts = append(s.stdoutParts, step.Stdout)
	s.commands = append(s.commands, step.Command)
	return step
}

func (s *stateRoundtripStateSession) fail(errs ...string) StepResult {
	return assertionStepWithOutput("state roundtrip", s.input.started, errs, s.stdoutParts, s.commands)
}

func (s *stateRoundtripStateSession) combineFailed(step StepResult) StepResult {
	return combineFailedStep("state roundtrip", s.input.started, step, s.stdoutParts, s.commands)
}

func (s *stateRoundtripStateSession) result(step StepResult) validateStateRoundtripStateResult {
	return validateStateRoundtripStateResult{step: step, stdoutParts: s.stdoutParts, commands: s.commands}
}

func (s *stateRoundtripStateSession) validateWriteReadList() StepResult {
	write := s.run("state write", s.input.binary, "state", "write", "--key", s.input.key, "--value", s.input.content, "--json")
	if !write.OK {
		return s.combineFailed(write)
	}
	var writeResult core.StateResult
	if err := json.Unmarshal([]byte(write.Stdout), &writeResult); err != nil {
		return s.fail(err.Error())
	}
	if !writeResult.OK || writeResult.Record.Key != s.input.key || writeResult.Record.Content != s.input.content || writeResult.Record.Bytes != len([]byte(s.input.content)) {
		return s.fail("write result did not match expected record")
	}

	read := s.run("state read", s.input.binary, "state", "read", "--key", s.input.key, "--json")
	if !read.OK {
		return s.combineFailed(read)
	}
	var readResult core.StateResult
	if err := json.Unmarshal([]byte(read.Stdout), &readResult); err != nil {
		return s.fail(err.Error())
	}
	if !readResult.OK || readResult.Record.Key != s.input.key || readResult.Record.Content != s.input.content || readResult.Record.Bytes != len([]byte(s.input.content)) {
		return s.fail("read result did not match expected record")
	}

	list := s.run("state list", s.input.binary, "state", "list", "--json")
	if !list.OK {
		return s.combineFailed(list)
	}
	var listResult core.StateListResult
	if err := json.Unmarshal([]byte(list.Stdout), &listResult); err != nil {
		return s.fail(err.Error())
	}
	if !listResult.OK || !containsString(listResult.Keys, s.input.key) {
		return s.fail("state list did not include roundtrip key")
	}
	return StepResult{OK: true}
}

func (s *stateRoundtripStateSession) validatePrune() StepResult {
	oldKey := s.input.key + "-old"
	oldWrite := s.run("state old write", s.input.binary, "state", "write", "--key", oldKey, "--value", "old state", "--json")
	if !oldWrite.OK {
		return s.combineFailed(oldWrite)
	}
	var oldWriteResult core.StateResult
	if err := json.Unmarshal([]byte(oldWrite.Stdout), &oldWriteResult); err != nil {
		return s.fail(err.Error())
	}
	oldWriteResult.Record.UpdatedAt = "2000-01-01T00:00:00Z"
	b, err := json.MarshalIndent(oldWriteResult.Record, "", "  ")
	if err != nil {
		return s.fail(err.Error())
	}
	if err := s.input.deps.writeFile(oldWriteResult.Path, append(b, '\n'), 0o600); err != nil {
		return s.fail(err.Error())
	}

	pruneDry := s.run("state prune dry-run", s.input.binary, "state", "prune", "--max-age", "1h", "--json")
	if !pruneDry.OK {
		return s.combineFailed(pruneDry)
	}
	var pruneDryResult core.StatePruneResult
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
	var pruneConfirmResult core.StatePruneResult
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
	var listAfterPruneResult core.StateListResult
	if err := json.Unmarshal([]byte(listAfterPrune.Stdout), &listAfterPruneResult); err != nil {
		return s.fail(err.Error())
	}
	if !containsString(listAfterPruneResult.Keys, s.input.key) || containsString(listAfterPruneResult.Keys, oldKey) {
		return s.fail("state prune did not preserve fresh key and remove old key")
	}
	return StepResult{OK: true}
}

func (s *stateRoundtripStateSession) validateMigrateAndDoctor() StepResult {
	legacyKey := s.input.key + "-legacy"
	legacyRecord := core.StateRecord{Key: legacyKey, Content: "legacy state", UpdatedAt: "2000-01-01T00:00:00Z", Bytes: len([]byte("legacy state"))}
	legacyBytes, err := json.MarshalIndent(legacyRecord, "", "  ")
	if err != nil {
		return s.fail(err.Error())
	}
	if err := s.input.deps.writeFile(filepath.Join(s.input.tempState, legacyKey+".json"), append(legacyBytes, '\n'), 0o600); err != nil {
		return s.fail(err.Error())
	}

	migrateDry := s.run("state migrate dry-run", s.input.binary, "state", "migrate", "--json")
	if !migrateDry.OK {
		return s.combineFailed(migrateDry)
	}
	var migrateDryResult core.StateMigrateResult
	if err := json.Unmarshal([]byte(migrateDry.Stdout), &migrateDryResult); err != nil {
		return s.fail(err.Error())
	}
	if !migrateDryResult.OK || !migrateDryResult.DryRun || !containsString(migrateDryResult.CandidateKeys, legacyKey) || len(migrateDryResult.MigratedKeys) != 0 {
		return s.fail("state migrate dry-run did not classify legacy key")
	}

	migrateConfirm := s.run("state migrate confirm", s.input.binary, "state", "migrate", "--confirm", "--json")
	if !migrateConfirm.OK {
		return s.combineFailed(migrateConfirm)
	}
	var migrateConfirmResult core.StateMigrateResult
	if err := json.Unmarshal([]byte(migrateConfirm.Stdout), &migrateConfirmResult); err != nil {
		return s.fail(err.Error())
	}
	if !migrateConfirmResult.OK || migrateConfirmResult.DryRun || !migrateConfirmResult.Confirm || !containsString(migrateConfirmResult.MigratedKeys, legacyKey) {
		return s.fail("state migrate confirm did not migrate legacy key")
	}

	migratedRead := s.run("state migrated read", s.input.binary, "state", "read", "--key", legacyKey, "--json")
	if !migratedRead.OK {
		return s.combineFailed(migratedRead)
	}
	var migratedReadResult core.StateResult
	if err := json.Unmarshal([]byte(migratedRead.Stdout), &migratedReadResult); err != nil {
		return s.fail(err.Error())
	}
	if migratedReadResult.Record.SchemaVersion != core.StateCurrentSchemaVersion || migratedReadResult.Record.Content != legacyRecord.Content {
		return s.fail("state migrate did not preserve content or set current schema")
	}

	doctorHealthy := s.run("state doctor after migrate", s.input.binary, "state", "doctor", "--json")
	if !doctorHealthy.OK {
		return s.combineFailed(doctorHealthy)
	}
	var doctorHealthyResult core.StateDoctorResult
	if err := json.Unmarshal([]byte(doctorHealthy.Stdout), &doctorHealthyResult); err != nil {
		return s.fail(err.Error())
	}
	if !doctorHealthyResult.OK || !doctorHealthyResult.Healthy {
		return s.fail("state doctor was not healthy after migrating legacy fixture")
	}
	return StepResult{OK: true}
}
