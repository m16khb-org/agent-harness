package hookcatalog

import (
	hookpromptcontract "agent-harness/internal/contract/hookprompt"
)

// prompt hint 구성은 저장소를 읽는다. 구현은 composition root가 설치한다.
var (
	BuildProjectDocCatalogContext func(repo string) hookpromptcontract.ProjectDocCatalogContext
)
