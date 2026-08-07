package draftwiki

import (
	"strings"
	"time"
)

func ProcessDraftWikiQueue(req DraftWikiQueueProcessRequest) (DraftWikiQueueProcessResult, error) {
	plan, path, err := draftWikiQueuePath(req.RepoRoot, true)
	if err != nil {
		return DraftWikiQueueProcessResult{OK: false}, err
	}
	result := DraftWikiQueueProcessResult{
		OK:              true,
		Kind:            "draft_wiki_queue_process",
		RepoRoot:        plan.RepoRoot,
		RepoID:          plan.RepoID,
		ProjectStateDir: plan.ProjectStateDir,
		Path:            path,
		Events:          []DraftWikiQueueEvent{},
	}
	unlock, locked, err := acquireDraftWikiQueueLock(plan.ProjectStateDir)
	if err != nil {
		return result, err
	}
	if !locked {
		result.Warnings = append(result.Warnings, "draft-wiki queue is already being processed")
		return result, nil
	}
	defer unlock()
	events, warnings, err := readDraftWikiQueueEvents(path)
	if err != nil {
		return DraftWikiQueueProcessResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: path}, err
	}
	result.Warnings = append(result.Warnings, warnings...)
	limit := req.Limit
	if limit <= 0 {
		limit = 1
	}
	for i := range events {
		if result.Processed >= limit {
			break
		}
		if events[i].Status != "" && events[i].Status != WorkerStatusQueued {
			continue
		}
		events[i].Status = WorkerStatusRunning
		events[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err := rewriteDraftWikiQueueEventsFunc(path, events); err != nil {
			return result, err
		}
		processed := processDraftWikiQueueEvent(req, events[i])
		events[i] = processed
		if err := rewriteDraftWikiQueueEventsFunc(path, events); err != nil {
			return result, err
		}
		result.Processed++
		if processed.Status == WorkerStatusSucceeded {
			result.Succeeded++
		} else {
			result.Failed++
		}
		result.Events = append(result.Events, draftWikiQueueEventForResponse(processed))
	}
	return result, nil
}

func processDraftWikiQueueEvent(req DraftWikiQueueProcessRequest, event DraftWikiQueueEvent) DraftWikiQueueEvent {
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		targetType = strings.TrimSpace(event.TargetType)
	}
	if targetType == "" {
		targetType = "notes"
	}
	targetWiki := strings.TrimSpace(req.TargetWiki)
	if targetWiki == "" {
		targetWiki = strings.TrimSpace(event.TargetWiki)
	}
	prompt := buildDraftWikiSuggestPrompt(DraftWikiSuggestRequest{
		Title:      "Draft wiki queued memory",
		TargetWiki: targetWiki,
		TargetType: targetType,
	}, event.SourceMaterial, targetType)
	event.Status = WorkerStatusSucceeded
	event.Prompt = prompt
	event.Error = ""
	event.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return event
}

func failDraftWikiQueueEvent(event DraftWikiQueueEvent, err error) DraftWikiQueueEvent {
	event.Status = WorkerStatusFailed
	event.Error = redactFreeform(err.Error())
	event.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return event
}

func draftWikiQueueEventForResponse(event DraftWikiQueueEvent) DraftWikiQueueEvent {
	event.SourceMaterial = ""
	return event
}
