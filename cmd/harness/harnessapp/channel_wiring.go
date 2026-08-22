package harnessapp

import (
	"agent-harness/cmd/harness/channelcli"
	channeladapter "agent-harness/internal/adapter/channel"
)

// channelDependencies는 channel CLI에 concrete adapter를 조립해 넘긴다.
func channelDependencies() channelcli.Dependencies {
	return channelcli.Dependencies{
		Send: channeladapter.Send,
		Recv: channeladapter.Recv,
	}
}
