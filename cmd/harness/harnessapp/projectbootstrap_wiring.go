package harnessapp

import (
	"agent-harness/cmd/harness/basiccli"
	"agent-harness/cmd/harness/mcpcli"
	"agent-harness/cmd/harness/projectcli"
	lifecycle "agent-harness/internal/adapter/lifecycle"
	"agent-harness/internal/adapter/projectbootstrap"
)

// projectbootstrap은 lifecycle state 초기화 구현을 모른다. 어댑터를 아는 곳은
// composition root 하나뿐이다.
func configureProjectBootstrap() {
	projectbootstrap.ConfigureLifecycle(lifecycle.InitProjectLifecycleState)
	basiccli.ConfigureProjectBootstrap(projectbootstrap.BootstrapProjectDocs)
	projectcli.ConfigureProjectBootstrap(projectbootstrap.BootstrapProjectDocs)
	mcpcli.ConfigureProjectBootstrap(projectbootstrap.BootstrapProjectDocs)
}
