package hookprompt

import (
	"strings"

	hookpromptcontract "agent-harness/internal/contract/hookprompt"
	projectdoc "agent-harness/internal/domain/projectdoc"
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
	b.WriteString("📚 agent-harness · 이 레포 project docs (관련된 것을 읽고 작업하세요)")
	for _, doc := range docs {
		name := strings.TrimPrefix(doc.RelPath, ".agent-harness/")
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
