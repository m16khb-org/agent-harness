package mcpcli

import (
	docscontract "agent-harness/internal/contract/docs"
)

// 문서 색인은 composition root가 설치한다. MCP tool router는 문서를 어떻게
// 훑는지 알지 않는다.
var DocsIndex func(root, version string) docscontract.DocsIndexResult
