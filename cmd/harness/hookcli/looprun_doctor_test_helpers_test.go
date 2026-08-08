package hookcli

import (
	doctorlg "agent-harness/internal/adapter/doctor"
	"agent-harness/internal/adapter/looprun"
)

// production wiring과 같은 loop gate 조회를 설치한다.
func init() {
	doctorlg.RepoGateSummaryFor = looprun.RepoGateSummaryFor
	doctorlg.LoopStateRoot = looprun.StateRoot
}
