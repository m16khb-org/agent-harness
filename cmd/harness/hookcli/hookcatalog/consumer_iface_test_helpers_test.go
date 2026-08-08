package hookcatalog

import (
	hookpromptadapter "agent-harness/internal/adapter/hookprompt"
)

// production wiring과 같은 구현을 설치한다.
func init() {
	BuildProjectDocCatalogContext = hookpromptadapter.BuildProjectDocCatalogContext
}
