package core

import (
	corestate "agent-harness/internal/core/state"
	"time"
)

const StateCurrentSchemaVersion = corestate.StateCurrentSchemaVersion

type StateRecord = corestate.StateRecord
type StateResult = corestate.StateResult
type StateListEntry = corestate.StateListEntry
type StateListResult = corestate.StateListResult
type StatePruneResult = corestate.StatePruneResult
type StateDoctorIssue = corestate.StateDoctorIssue
type StateDoctorResult = corestate.StateDoctorResult
type StateMigrateResult = corestate.StateMigrateResult

func StateDir() string {
	return corestate.StateDir()
}

func NormalizeStateKey(key string) (string, error) {
	return corestate.NormalizeStateKey(key)
}

func StateWrite(key, content string) (StateResult, error) {
	return corestate.StateWrite(key, content)
}

func StateRead(key string) (StateResult, error) {
	return corestate.StateRead(key)
}

func StateList() (StateListResult, error) {
	return corestate.StateList()
}

func StateDoctor() (StateDoctorResult, error) {
	return corestate.StateDoctor()
}

func StatePrune(maxAge time.Duration, confirm bool) (StatePruneResult, error) {
	return corestate.StatePrune(maxAge, confirm)
}

func StateMigrate(confirm bool) (StateMigrateResult, error) {
	return corestate.StateMigrate(confirm)
}
