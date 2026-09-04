package issueopsapp

import (
	"issueops/cmd/issueops/hookcli/hookcatalog"
	hookpromptadapter "issueops/internal/adapter/hookprompt"
)

// configureHookPrompts는 context hook의 project-doc catalog 구성을 설치한다.
func configureHookPrompts() {
	hookcatalog.BuildProjectDocCatalogContext = hookpromptadapter.BuildProjectDocCatalogContext
}
