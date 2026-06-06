package validationcli

import (
	"encoding/json"
	"path/filepath"

	"agent-harness/internal/core"
)

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
