package projectbootstrap

import (
	"agent-harness/internal/adapter/draftwiki"
)

// production wiring과 같은 draft wiki 초기화를 설치한다.
func init() {
	InitDraftWiki = draftwiki.InitDraftWiki
}
