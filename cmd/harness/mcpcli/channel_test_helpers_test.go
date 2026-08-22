package mcpcli

import (
	"agent-harness/internal/adapter/channel"
	gatesadapter "agent-harness/internal/adapter/gates"
	"agent-harness/internal/adapter/outbound/sqlstore"
	statestore "agent-harness/internal/adapter/outbound/state"
	policyadapter "agent-harness/internal/adapter/policy"
)

// production wiring과 같은 channel/gates adapter 구현을 설치한다.
func init() {
	ChannelSend = channel.Send
	ChannelRecv = channel.Recv
	channel.StateDir = statestore.StateDir
	channel.GetExisting = sqlstore.GetExisting
	channel.ListExisting = sqlstore.ListExisting
	channel.OpenStateDatabase = func(dir string) (channel.StateDatabase, error) { return sqlstore.Open(dir) }

	GatesCheck = gatesadapter.Check
	GatesInit = gatesadapter.Init
	GatesAbandon = gatesadapter.Abandon
	gatesadapter.EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
	gatesadapter.RunCommand = policyadapter.RunCommand
}
