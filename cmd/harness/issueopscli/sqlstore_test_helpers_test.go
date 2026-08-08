package issueopscli

import (
	looprunsqld "agent-harness/internal/adapter/looprun"
	"agent-harness/internal/adapter/outbound/sqlstore"
)

// production wiring과 같은 저장소를 설치한다.
func init() {
	looprunsqld.GetExisting = sqlstore.GetExisting
	looprunsqld.ListExisting = sqlstore.ListExisting
	looprunsqld.OpenStateDatabase = func(dir string) (looprunsqld.StateDatabase, error) { return sqlstore.Open(dir) }
}
