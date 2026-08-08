package hookcatalog

import (
	hookpromptpddeps "agent-harness/internal/adapter/hookprompt"
	projectbootstrappddeps "agent-harness/internal/adapter/projectbootstrap"
	projectdocadapter "agent-harness/internal/adapter/projectdoc"
	projectdocspddeps "agent-harness/internal/adapter/projectdocs"
)

// production wiring과 같은 문서 reader를 설치한다.
func init() {
	hookpromptpddeps.DiscoverProjectDocs = projectdocadapter.DiscoverProjectDocs
	hookpromptpddeps.FormatProjectDocCatalog = projectdocadapter.FormatProjectDocCatalog
	projectbootstrappddeps.PlannedFileAction = projectdocadapter.PlannedFileAction
	projectdocspddeps.PlannedFileAction = projectdocadapter.PlannedFileAction
}
