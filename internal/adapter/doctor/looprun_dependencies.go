package doctor

import (
	loopruncontract "agent-harness/internal/contract/looprun"
)

// loop gate 요약과 상태 경로는 harness state를 읽는다. 구현은 composition root가
// 설치한다.
var (
	RepoGateSummaryFor func(repo string) (loopruncontract.RepoGateSummary, []string)
	LoopStateRoot      func() string
)
