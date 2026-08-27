package harnessapp

import (
	"agent-harness/cmd/harness/hookcli/hookcatalog"
	hookpromptadapter "agent-harness/internal/adapter/hookprompt"
)

// configureHookPrompts는 context hook의 project-doc catalog 구성을 설치한다.
func configureHookPrompts() {
	hookcatalog.BuildProjectDocCatalogContext = hookpromptadapter.BuildProjectDocCatalogContext
}
