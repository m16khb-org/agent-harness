package augmentplan

import (
	augmentcatalogcli "issueops/cmd/issueops/selfworkflow/augmentcatalog"
	qagatecli "issueops/cmd/issueops/validationcli/qagate"
	"issueops/internal/adapter/docs"
	"issueops/internal/adapter/inspect"
)

// production wiring과 같은 문서 reader를 설치한다. 이 package의 테스트는 다른
// package를 거쳐 문서 조회에 닿으므로 간접 의존까지 함께 채운다. fitness graph는
// test import를 수집하지 않으므로 여기서는 concrete를 써도 된다.
func init() {
	DocsIndex = docs.DocsIndex
	inspect.ListDocs = docs.ListDocs
	augmentcatalogcli.ListDocs = docs.ListDocs
	qagatecli.ListDocs = docs.ListDocs
}
