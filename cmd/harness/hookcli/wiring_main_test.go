package hookcli

import (
	"os"
	"testing"

	"agent-harness/cmd/harness/hookcli/hookcatalog"
	"agent-harness/cmd/harness/hookcli/hookenv"
	hookpromptadapter "agent-harness/internal/adapter/hookprompt"
	projectdocadapter "agent-harness/internal/adapter/projectdoc"
)

// 프로덕션에서는 harnessapp이 주입한다. 상속된 운영자 스위치(HARNESS_DISABLE_HOOKS)
// 는 먼저 지운다 — dogfood 셸의 값이 새어 들어오면 context hook이 아무것도
// 내보내지 않아 테스트 결론이 바뀐다(#395).
func TestMain(m *testing.M) {
	hookenv.ClearInheritedOperatorSwitches()
	hookcatalog.BuildProjectDocCatalogContext = hookpromptadapter.BuildProjectDocCatalogContext
	hookpromptadapter.DiscoverProjectDocs = projectdocadapter.DiscoverProjectDocs
	hookpromptadapter.FormatProjectDocCatalog = projectdocadapter.FormatProjectDocCatalog
	os.Exit(m.Run())
}
