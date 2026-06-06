package hookprompt

import "strings"

type ProjectDocCatalogContext struct {
	ShouldInject bool
	ProjectDocs  []ProjectDocCatalogEntry
	Compact      string
	UserView     string
}

func BuildProjectDocCatalogContext(repo string) ProjectDocCatalogContext {
	docs := DiscoverProjectDocs(repo)
	if len(docs) == 0 {
		return ProjectDocCatalogContext{}
	}
	return ProjectDocCatalogContext{
		ShouldInject: true,
		ProjectDocs:  docs,
		Compact:      FormatProjectDocCatalog(docs),
		UserView:     renderProjectDocCatalogUserView(docs),
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

func RenderUserPromptUserView(result HookUserPromptResult) string {
	return renderProjectDocCatalogUserView(result.ProjectDocs)
}

func RenderUserPromptCodexContext(result HookUserPromptResult) string {
	return RenderUserPromptUserView(result)
}
