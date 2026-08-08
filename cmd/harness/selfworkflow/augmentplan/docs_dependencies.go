package augmentplan

import (
	docscontract "agent-harness/internal/contract/docs"
)

// 문서 색인은 composition root가 설치한다.
var DocsIndex func(root, version string) docscontract.DocsIndexResult
