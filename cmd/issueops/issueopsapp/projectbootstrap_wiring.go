package issueopsapp

import (
	"issueops/cmd/issueops/mcpcli"
	"issueops/cmd/issueops/projectcli"
	lifecycle "issueops/internal/adapter/lifecycle"
	"issueops/internal/adapter/projectbootstrap"
)

// projectbootstrap은 lifecycle state 초기화 구현을 모른다. 어댑터를 아는 곳은
// composition root 하나뿐이다.
func configureProjectBootstrap() {
	projectbootstrap.ConfigureLifecycle(lifecycle.InitProjectLifecycleState)
	projectcli.ConfigureProjectBootstrap(projectbootstrap.BootstrapProjectDocs)
	mcpcli.ConfigureProjectBootstrap(projectbootstrap.BootstrapProjectDocs)
}
