package core

import coreguard "agent-harness/internal/core/guard"

type GuardCheckRequest = coreguard.GuardCheckRequest
type GuardCheckResult = coreguard.GuardCheckResult
type GuardFinding = coreguard.GuardFinding
type GuardSummary = coreguard.GuardSummary
type GuardBlockedError = coreguard.GuardBlockedError

func GuardCheck(req GuardCheckRequest) GuardCheckResult {
	return coreguard.GuardCheck(req)
}

func IsGuardBlocked(err error) bool {
	return coreguard.IsGuardBlocked(err)
}
