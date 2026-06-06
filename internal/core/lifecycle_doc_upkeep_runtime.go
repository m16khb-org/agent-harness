package core

import (
	"fmt"
	"strings"
)

func RecordLifecycleToolUse(req HookToolUseLifecycleRequest) (HookToolUseLifecycleResult, error) {
	repo := strings.TrimSpace(req.Repo)
	if repo == "" {
		return HookToolUseLifecycleResult{OK: true, Warnings: []string{"repo_missing"}}, nil
	}
	targets := lifecycleDocTargetsForToolUse(req)
	if len(targets) == 0 {
		return HookToolUseLifecycleResult{OK: true}, nil
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "post-tool-use"
	}
	summary := "Relevant harness files changed; shared project docs may need review."
	if req.Tool != "" {
		summary = fmt.Sprintf("%s touched harness lifecycle-relevant files; shared project docs may need review.", req.Tool)
	}
	appendResult, err := AppendDocUpkeepEvent(repo, DocUpkeepEvent{
		Kind:       "code_change",
		TargetDocs: targets,
		Summary:    summary,
		Evidence:   append([]string{}, req.Paths...),
		Source:     source,
	})
	if err != nil {
		return HookToolUseLifecycleResult{OK: false}, err
	}
	return HookToolUseLifecycleResult{OK: true, Recorded: true, Event: appendResult.Event}, nil
}

func BuildLifecycleStopReminder(repo string) LifecycleStopReminderResult {
	events, _, err := ReadPendingDocUpkeepEvents(repo, 5)
	if err != nil || len(events) == 0 {
		return LifecycleStopReminderResult{OK: true}
	}
	var b strings.Builder
	b.WriteString("Pending .agent-harness doc upkeep:\n")
	for _, event := range events {
		b.WriteString("- ")
		if len(event.TargetDocs) > 0 {
			b.WriteString(strings.Join(event.TargetDocs, ", "))
			b.WriteString(": ")
		}
		b.WriteString(event.Summary)
		b.WriteString("\n")
	}
	b.WriteString("Use project_docs_record for ADR/caution entries or project_docs_read/project_docs_update for evidence-preserving doc refreshes.")
	return LifecycleStopReminderResult{OK: true, ShouldInject: true, AdditionalContext: strings.TrimSpace(b.String()), PendingCount: len(events)}
}
