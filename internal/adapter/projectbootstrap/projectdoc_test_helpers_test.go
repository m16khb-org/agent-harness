package projectbootstrap

import (
	projectdocadapter "issueops/internal/adapter/projectdoc"
	projectdocspddeps "issueops/internal/adapter/projectdocs"
)

// production wiring과 같은 문서 reader를 설치한다.
func init() {
	PlannedFileAction = projectdocadapter.PlannedFileAction
	projectdocspddeps.PlannedFileAction = projectdocadapter.PlannedFileAction
}
