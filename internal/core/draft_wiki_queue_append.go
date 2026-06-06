package core

import (
	"fmt"
	"strings"
	"time"
)

func AppendDraftWikiQueueEvent(req DraftWikiQueueAppendRequest) (DraftWikiQueueAppendResult, error) {
	plan, path, err := draftWikiQueuePath(req.RepoRoot, true)
	if err != nil {
		return DraftWikiQueueAppendResult{OK: false}, err
	}
	material := trimDraftWikiQueueMaterial(req.SourceMaterial)
	if material == "" {
		return DraftWikiQueueAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: path}, fmt.Errorf("draft wiki queue source material is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		targetType = "notes"
	}
	event := DraftWikiQueueEvent{
		ID:             draftWikiQueueEventID(plan.RepoID, material, now),
		Kind:           "draft_wiki_suggest",
		RepoRoot:       plan.RepoRoot,
		Tool:           strings.TrimSpace(req.Tool),
		Command:        redactFreeform(strings.TrimSpace(req.Command)),
		Paths:          redactStringSlice(req.Paths),
		SourceMaterial: material,
		TargetWiki:     strings.TrimSpace(req.TargetWiki),
		TargetType:     targetType,
		Source:         strings.TrimSpace(req.Source),
		Status:         WorkerStatusQueued,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if event.Source == "" {
		event.Source = "main-agent"
	}
	if err := appendDraftWikiQueueEvent(path, event); err != nil {
		return DraftWikiQueueAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: path}, err
	}
	if err := capDraftWikiQueueEvents(path, maxDraftWikiQueueEvents); err != nil {
		return DraftWikiQueueAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: path}, err
	}
	return DraftWikiQueueAppendResult{OK: true, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: path, Event: draftWikiQueueEventForResponse(event)}, nil
}
