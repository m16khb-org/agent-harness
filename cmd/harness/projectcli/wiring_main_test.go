package projectcli

import (
	lifecycle "agent-harness/internal/adapter/lifecycle"
	"agent-harness/internal/adapter/projectbootstrap"
	"os"
	"testing"
)

// 프로덕션에서는 harnessapp이 주입한다. 이 CLI 테스트는 실제 부트스트랩 동작을
// 검증하므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {
	projectbootstrap.ConfigureLifecycle(lifecycle.InitProjectLifecycleState)
	ConfigureProjectBootstrap(projectbootstrap.BootstrapProjectDocs)
	os.Exit(m.Run())
}
