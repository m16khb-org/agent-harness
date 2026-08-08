package hookprompt

import (
	projectbootstrappddeps "agent-harness/internal/adapter/projectbootstrap"
	projectdocadapter "agent-harness/internal/adapter/projectdoc"
	projectdocspddeps "agent-harness/internal/adapter/projectdocs"
)

// production wiring과 같은 문서 reader를 설치한다.
func init() {
	DiscoverProjectDocs = projectdocadapter.DiscoverProjectDocs
	FormatProjectDocCatalog = projectdocadapter.FormatProjectDocCatalog
	projectbootstrappddeps.PlannedFileAction = projectdocadapter.PlannedFileAction
	projectdocspddeps.PlannedFileAction = projectdocadapter.PlannedFileAction
}
