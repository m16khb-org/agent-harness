package workercli

import (
	"issueops/internal/adapter/outbound/sqlstore"
	workersqld "issueops/internal/adapter/worker"
)

// production wiring과 같은 저장소를 설치한다.
func init() {
	workersqld.OpenStateDatabase = func(dir string) (workersqld.StateDatabase, error) { return sqlstore.Open(dir) }
}
