package inspect

import (
	"agent-harness/internal/adapter/docs"
)

// production wiring과 같은 문서 reader를 설치한다.
func init() {
	ListDocs = docs.ListDocs
}
