package core

import (
	"time"

	corepreflight "agent-harness/internal/core/preflight"
	corestate "agent-harness/internal/core/state"
	coretrace "agent-harness/internal/core/trace"
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

func WriteStateRecord(dir, key string, record StateRecord) (string, error) {
	return corestate.WriteStateRecord(dir, key, record)
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

func StatePrunePrefix(prefix string, maxAge time.Duration, maxRecords int, confirm bool) (StatePruneResult, error) {
	return corestate.StatePrunePrefix(prefix, maxAge, maxRecords, confirm)
}

func StateMigrate(confirm bool) (StateMigrateResult, error) {
	return corestate.StateMigrate(confirm)
}

func StateDelete(key string) error {
	return corestate.StateDelete(key)
}

type StateMaintainResult = corestate.StateMaintainResult

func StateMaintain() (StateMaintainResult, error) {
	return corestate.StateMaintain()
}

const TraceAnalysisKind = coretrace.TraceAnalysisKind

type TraceAnalyzeRequest = coretrace.TraceAnalyzeRequest
type TraceAnalyzeResult = coretrace.TraceAnalyzeResult
type TraceAnalysisFinding = coretrace.TraceAnalysisFinding

func TraceAnalyze(req TraceAnalyzeRequest) (TraceAnalyzeResult, error) {
	return coretrace.TraceAnalyze(req)
}

type PreflightResult = corepreflight.PreflightResult
type RemoteInfo = corepreflight.RemoteInfo
type CommitInfo = corepreflight.CommitInfo

func GitPreflight(target, harnessRoot string) PreflightResult {
	return corepreflight.GitPreflight(target, harnessRoot)
}

func GitCmd(dir string, args ...string) (int, string, string) {
	return corepreflight.GitCmd(dir, args...)
}

func GitOut(dir string, args ...string) string {
	return corepreflight.GitOut(dir, args...)
}
