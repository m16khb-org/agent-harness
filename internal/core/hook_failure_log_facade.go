package core

import "agent-harness/internal/core/hookfailure"

type HookFailureEvent = hookfailure.HookFailureEvent
type HookFailureRecordResult = hookfailure.HookFailureRecordResult
type HookFailureListResult = hookfailure.HookFailureListResult

func RecordHookFailureEvent(event HookFailureEvent) (HookFailureRecordResult, error) {
	return hookfailure.RecordHookFailureEvent(event)
}

func ListHookFailureEvents(limit int) (HookFailureListResult, error) {
	return hookfailure.ListHookFailureEvents(limit)
}

func HookFailureLogPath() string {
	return hookfailure.HookFailureLogPath()
}
