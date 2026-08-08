package projectbootstrap

import (
	draftwikicontract "agent-harness/internal/contract/draftwiki"
)

// draft wiki 초기화는 디스크에 파일을 만든다. 구현은 composition root가 설치한다.
var InitDraftWiki func(req draftwikicontract.DraftWikiInitRequest) (draftwikicontract.DraftWikiInitResult, error)
