package state

import (
	"time"

	statecontract "issueops/internal/contract/state"
)

func StatePrune(maxAge time.Duration, confirm bool) (statecontract.StatePruneResult, error) {
	return service().Prune(maxAge, confirm)
}

func StatePrunePrefix(prefix string, maxAge time.Duration, maxRecords int, confirm bool) (statecontract.StatePruneResult, error) {
	return service().PrunePrefix(prefix, maxAge, maxRecords, confirm)
}
