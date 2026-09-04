package statuscli

import (
	looprunsqld "issueops/internal/adapter/looprun"
	"issueops/internal/adapter/outbound/sqlstore"
	workersqld "issueops/internal/adapter/worker"
)

// production wiring과 같은 저장소를 설치한다.
func init() {
	looprunsqld.GetExisting = sqlstore.GetExisting
	looprunsqld.ListExisting = sqlstore.ListExisting
	looprunsqld.OpenStateDatabase = func(dir string) (looprunsqld.StateDatabase, error) { return sqlstore.Open(dir) }
	workersqld.OpenStateDatabase = func(dir string) (workersqld.StateDatabase, error) { return sqlstore.Open(dir) }
}
