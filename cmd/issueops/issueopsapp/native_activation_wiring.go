package issueopsapp

import (
	nativeactivationoutbound "issueops/internal/adapter/outbound/nativeactivation"
	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/port"
	activationport "issueops/internal/port/nativeactivation"
)

func nativeActivationBackend() activationport.Backend {
	return nativeactivationoutbound.NewBackend(func(root string) (port.TransactionalRecordStore, error) {
		return sqlstore.Open(root)
	})
}
