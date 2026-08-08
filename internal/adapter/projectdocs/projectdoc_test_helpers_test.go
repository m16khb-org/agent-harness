package projectdocs

import (
	projectdocadapter "agent-harness/internal/adapter/projectdoc"
)

// production wiring과 같은 문서 reader를 설치한다.
func init() {
	PlannedFileAction = projectdocadapter.PlannedFileAction
}
