package looprun

import (
	"agent-harness/internal/adapter/outbound/sqlstore"
)

// production wiring과 같은 저장소를 설치한다.
func init() {
	GetExisting = sqlstore.GetExisting
	ListExisting = sqlstore.ListExisting
	OpenStateDatabase = func(dir string) (StateDatabase, error) { return sqlstore.Open(dir) }
}
