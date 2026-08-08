package doctor

import (
	"agent-harness/internal/adapter/draftwiki"
	pbdeps "agent-harness/internal/adapter/projectbootstrap"
)

// production wiring과 같은 draft wiki 초기화를 설치한다.
func init() {
	pbdeps.InitDraftWiki = draftwiki.InitDraftWiki
}
