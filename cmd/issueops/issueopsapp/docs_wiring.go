package issueopsapp

import (
	"issueops/cmd/issueops/mcpcli"
	"issueops/cmd/issueops/selfworkflow/augmentcatalog"
	"issueops/cmd/issueops/selfworkflow/augmentplan"
	"issueops/cmd/issueops/validationcli/qagate"
	"issueops/internal/adapter/docs"
	"issueops/internal/adapter/inspect"
)

// configureDocsReaders는 문서 색인·목록·heading 읽기를 설치한다.
//
// 문서를 어떻게 훑는지는 하나의 구현이고, 그 선택은 composition root의 결정이다.
// 소비자들은 transport와 조립 순서가 서로 다르므로 package 변수로 설치한다 —
// Deps를 중간 package로 전달하면 그 package가 대신 문서 구현을 알게 된다.
func configureDocsReaders() {
	mcpcli.DocsIndex = docs.DocsIndex
	augmentplan.DocsIndex = docs.DocsIndex
	augmentcatalog.ListDocs = docs.ListDocs
	qagate.ListDocs = docs.ListDocs
	inspect.ListDocs = docs.ListDocs
}
