package hookcatalog

import (
	hookpromptpddeps "issueops/internal/adapter/hookprompt"
	projectdocadapter "issueops/internal/adapter/projectdoc"
)

// production wiring과 같은 문서 reader를 설치한다.
func init() {
	hookpromptpddeps.DiscoverProjectDocs = projectdocadapter.DiscoverProjectDocs
	hookpromptpddeps.FormatProjectDocCatalog = projectdocadapter.FormatProjectDocCatalog
}
