package mcpcli

import (
	commitsuggestcontract "issueops/internal/contract/commitsuggest"
)

// 이 연산은 실제 I/O를 수행한다. 구현은 composition root가 설치한다.
var (
	SuggestCommit func(req commitsuggestcontract.CommitSuggestRequest) (commitsuggestcontract.CommitSuggestResult, error)
)
