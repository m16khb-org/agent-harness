package augmentcatalog

import (
	"issueops/internal/adapter/docs"
)

// production wiring과 같은 문서 reader를 설치한다. augmentplan이 이 package를
// import하므로 여기서 역방향으로 채우면 순환이 된다 — 자기 것만 설치한다.
func init() {
	ListDocs = docs.ListDocs
}
