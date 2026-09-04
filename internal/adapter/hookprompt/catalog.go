package hookprompt

import (
	"strings"

	hookpromptcontract "issueops/internal/contract/hookprompt"
	projectdoc "issueops/internal/domain/projectdoc"
)

// ProjectDocCatalogEntry is the domain catalog entry the hook renders.
type ProjectDocCatalogEntry = projectdoc.ProjectDocCatalogEntry

func BuildProjectDocCatalogContext(repo string) hookpromptcontract.ProjectDocCatalogContext {
	docs := DiscoverProjectDocs(repo)
	if len(docs) == 0 {
		return hookpromptcontract.ProjectDocCatalogContext{}
	}
	compact := FormatProjectDocCatalog(docs)
	userView := renderProjectDocCatalogUserView(docs)
	return hookpromptcontract.ProjectDocCatalogContext{
		ShouldInject: true,
		ProjectDocs:  docs,
		Compact:      compact,
		UserView:     userView,
	}
}

func renderProjectDocCatalogUserView(docs []ProjectDocCatalogEntry) string {
	if len(docs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("📚 issueops · 이 레포 project docs (관련된 것을 읽고 작업하세요)")
	for _, doc := range docs {
		name := strings.TrimPrefix(doc.RelPath, ".issueops/")
		desc := doc.Description
		if desc == "" {
			desc = doc.Title
		}
		b.WriteString("\n• " + name)
		if desc != "" {
			b.WriteString(" — " + desc)
		}
	}
	return b.String()
}
