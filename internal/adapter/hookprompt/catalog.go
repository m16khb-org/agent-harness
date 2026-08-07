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
	worktreeReminder := activeWorktreeReminderValue(repo)
	orchestrationReminder := orchestrationReminderValue(repo)
	if len(docs) == 0 && worktreeReminder == "" && orchestrationReminder == "" {
		return ProjectDocCatalogContext{}
	}
	compact := FormatProjectDocCatalog(docs)
	userView := renderProjectDocCatalogUserView(docs)
	if worktreeReminder != "" {
		compact = appendCatalogContextLine(compact, "worktree: "+worktreeReminder)
		userView = appendCatalogContextLine(userView, "• worktree: "+worktreeReminder)
	}
	for _, line := range strings.Split(orchestrationReminder, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		compact = appendCatalogContextLine(compact, "orchestration: "+line)
		userView = appendCatalogContextLine(userView, "• orchestration: "+line)
	}
	return ProjectDocCatalogContext{
		ShouldInject: true,
		ProjectDocs:  docs,
		Compact:      compact,
		UserView:     userView,
	}
}

func appendCatalogContextLine(context, line string) string {
	context = strings.TrimSpace(context)
	line = strings.TrimSpace(line)
	if context == "" {
		return line
	}
	if line == "" {
		return context
	}
	return context + "\n" + line
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
