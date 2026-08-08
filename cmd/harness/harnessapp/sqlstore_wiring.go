package harnessapp

import (
	"agent-harness/cmd/harness/validationcli/stateroundtrip"
	"agent-harness/internal/adapter/looprun"
	"agent-harness/internal/adapter/outbound/sqlstore"
	"agent-harness/internal/adapter/worker"
)

// configureStateDatabases는 SQLite 저장소를 각 소비자에 조립한다.
//
// 소비자들은 자기가 쓰는 연산만 인터페이스로 선언한다. 어떤 엔진이 그 연산을
// 수행하는지는 composition root의 결정이다.
func configureStateDatabases() {
	looprun.OpenStateDatabase = func(dir string) (looprun.StateDatabase, error) { return sqlstore.Open(dir) }
	looprun.GetExisting = sqlstore.GetExisting
	looprun.ListExisting = sqlstore.ListExisting
	worker.OpenStateDatabase = func(dir string) (worker.StateDatabase, error) { return sqlstore.Open(dir) }
	stateroundtrip.OpenStateDatabase = func(dir string) (stateroundtrip.StateDatabase, error) { return sqlstore.Open(dir) }
}
