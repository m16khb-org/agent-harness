package state

import (
	"issueops/internal/adapter/outbound/sqlstore"
	stateapplication "issueops/internal/application/state"
	statecontract "issueops/internal/contract/state"
	"issueops/internal/domain/statepath"
	stateport "issueops/internal/port/state"
)

const stateBucket = "state"

func StateDir() string {
	return stateDir()
}

func NormalizeStateKey(key string) (string, error) {
	return statepath.NormalizeKey(key)
}

func statePath(dir, key string) string {
	return statepath.Path(dir, key)
}

func openStateDB(dir string) (*sqlstore.DB, error) {
	return sqlstore.Open(dir)
}

func openStateStore(dir string) (stateport.Store, error) {
	return sqlstore.Open(dir)
}

type existingRecords struct{}

func (existingRecords) GetExisting(dir, bucket, id string) ([]byte, bool, error) {
	return sqlstore.GetExisting(dir, bucket, id)
}

var _ stateport.ExistingReader = existingRecords{}

func service() *stateapplication.Service {
	return stateapplication.NewService(stateapplication.Dependencies{
		StateDir:        stateDir,
		StatePath:       statepath.Path,
		OpenStore:       openStateStore,
		ExistingRecords: existingRecords{},
	})
}

func StateWrite(key, content string) (statecontract.StateResult, error) {
	return service().Write(key, content)
}

func StateRead(key string) (statecontract.StateResult, error) {
	return service().Read(key)
}

func StateList() (statecontract.StateListResult, error) {
	return service().List()
}

func WriteStateRecord(dir, key string, record statecontract.RecordEnvelope) (string, error) {
	return service().WriteRecord(dir, key, record)
}

func StateDelete(key string) error {
	return service().Delete(key)
}
