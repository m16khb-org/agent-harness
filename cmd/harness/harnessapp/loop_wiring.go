package harnessapp

import (
	"agent-harness/cmd/harness/loopcli"
	"agent-harness/internal/adapter/looprun"
)

// loopDependencies는 loop CLI에 concrete loop run store를 조립해 넘긴다.
func loopDependencies() loopcli.Dependencies {
	return loopcli.Dependencies{
		Start:         looprun.Start,
		RecordAttempt: looprun.RecordAttempt,
		Stop:          looprun.Stop,
		Status:        looprun.Status,
		ResolveID:     looprun.ResolveID,
	}
}
