package harnessapp

import (
	"agent-harness/internal/adapter/draftwiki"
	"agent-harness/internal/adapter/projectbootstrap"
)

// configureDraftWikiWriters는 draft wiki 초기화를 설치한다.
func configureDraftWikiWriters() {
	projectbootstrap.InitDraftWiki = draftwiki.InitDraftWiki
}
