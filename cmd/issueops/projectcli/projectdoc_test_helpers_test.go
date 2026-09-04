package projectcli

import (
	projectbootstrappddeps "issueops/internal/adapter/projectbootstrap"
	projectdocadapter "issueops/internal/adapter/projectdoc"
	projectdocspddeps "issueops/internal/adapter/projectdocs"
)

// production wiring과 같은 문서 reader를 설치한다.
func init() {
	projectbootstrappddeps.PlannedFileAction = projectdocadapter.PlannedFileAction
	projectdocspddeps.PlannedFileAction = projectdocadapter.PlannedFileAction
}
