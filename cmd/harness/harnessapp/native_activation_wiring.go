package harnessapp

import (
	nativeactivationoutbound "agent-harness/internal/adapter/outbound/nativeactivation"
	"agent-harness/internal/adapter/outbound/sqlstore"
	"agent-harness/internal/port"
	activationport "agent-harness/internal/port/nativeactivation"
)

func nativeActivationBackend() activationport.Backend {
	return nativeactivationoutbound.NewBackend(func(root string) (port.TransactionalRecordStore, error) {
		return sqlstore.Open(root)
	})
}
