package channelcli

import (
	channeladapter "agent-harness/internal/adapter/channel"
	"agent-harness/internal/adapter/outbound/sqlstore"
	statestore "agent-harness/internal/adapter/outbound/state"
)

// 실제 adapter 구현을 설치한다. 저장소 배선은 production wiring과 같다.
var (
	adapterSend = channeladapter.Send
	adapterRecv = channeladapter.Recv
)

func init() {
	channeladapter.StateDir = statestore.StateDir
	channeladapter.GetExisting = sqlstore.GetExisting
	channeladapter.ListExisting = sqlstore.ListExisting
	channeladapter.OpenStateDatabase = func(dir string) (channeladapter.StateDatabase, error) { return sqlstore.Open(dir) }
}
