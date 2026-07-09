package core

import "agent-harness/internal/core/looprun"

type LoopRun = looprun.LoopRun
type LoopAttempt = looprun.LoopAttempt
type LoopRunStartRequest = looprun.StartLoopRequest
type LoopRunRecordAttemptRequest = looprun.RecordAttemptRequest
type LoopRunStatusResult = looprun.StatusResult

func StartLoopRun(req LoopRunStartRequest) (LoopRun, error) {
	return looprun.Start(req)
}

func RecordLoopAttempt(id string, req LoopRunRecordAttemptRequest) (LoopRun, error) {
	return looprun.RecordAttempt(id, req)
}

func StopLoopRun(id string, success bool, reason string) (LoopRun, error) {
	return looprun.Stop(id, success, reason)
}

func LoopRunStatus(id string) (LoopRunStatusResult, error) {
	return looprun.Status(id)
}

func ResolveLoopRunID(repo, name string) (string, error) {
	return looprun.ResolveID(repo, name)
}
