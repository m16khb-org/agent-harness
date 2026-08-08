package harnessapp

import (
	"agent-harness/internal/adapter/hookprompt"
	"agent-harness/internal/adapter/projectbootstrap"
	"agent-harness/internal/adapter/projectdoc"
	"agent-harness/internal/adapter/projectdocs"
)

// configureProjectDocReaders는 프로젝트 문서 탐색과 파일 상태 판정을 설치한다.
//
// 문서 이름·마커 같은 규칙은 domain이 소유하므로 소비자가 직접 import한다.
// 디스크를 읽는 부분만 여기서 조립한다.
func configureProjectDocReaders() {
	hookprompt.DiscoverProjectDocs = projectdoc.DiscoverProjectDocs
	hookprompt.FormatProjectDocCatalog = projectdoc.FormatProjectDocCatalog
	projectbootstrap.PlannedFileAction = projectdoc.PlannedFileAction
	projectdocs.PlannedFileAction = projectdoc.PlannedFileAction
}
