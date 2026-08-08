package doctor

import (
	"agent-harness/internal/adapter/looprun"
)

// production wiring과 같은 loop gate 조회를 설치한다.
func init() {
	RepoGateSummaryFor = looprun.RepoGateSummaryFor
	LoopStateRoot = looprun.StateRoot
}
