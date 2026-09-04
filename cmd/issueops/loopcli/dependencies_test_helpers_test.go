package loopcli

import (
	"issueops/internal/adapter/looprun"
)

// testDependencies는 production wiring과 같은 loop run store를 조립한다.
// fitness graph는 test import를 수집하지 않으므로 concrete를 써도 된다.
func testDependencies() Dependencies {
	return Dependencies{
		Start:         looprun.Start,
		RecordAttempt: looprun.RecordAttempt,
		Stop:          looprun.Stop,
		Status:        looprun.Status,
		ResolveID:     looprun.ResolveID,
	}
}
