package stateroundtrip

import (
	"issueops/internal/adapter/outbound/sqlstore"
)

// production wiring과 같은 저장소를 설치한다.
func init() {
	OpenStateDatabase = func(dir string) (StateDatabase, error) { return sqlstore.Open(dir) }
}
