package core

import (
	"time"
)

const draftWikiQueueFile = "draft-wiki-queue.jsonl"

const draftWikiQueueLockFile = "draft-wiki-queue.lock"

const maxDraftWikiQueueEvents = 200

type DraftWikiQueueAppendRequest struct {
	RepoRoot       string   `json:"repo_root"`
	Tool           string   `json:"tool,omitempty"`
	Command        string   `json:"command,omitempty"`
	Paths          []string `json:"paths,omitempty"`
	SourceMaterial string   `json:"source_material"`
	TargetWiki     string   `json:"target_wiki,omitempty"`
	TargetType     string   `json:"target_type,omitempty"`
	Source         string   `json:"source,omitempty"`
}

type DraftWikiQueueEvent struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	RepoRoot       string   `json:"repo_root"`
	Tool           string   `json:"tool,omitempty"`
	Command        string   `json:"command,omitempty"`
	Paths          []string `json:"paths,omitempty"`
	SourceMaterial string   `json:"source_material,omitempty"`
	TargetWiki     string   `json:"target_wiki,omitempty"`
	TargetType     string   `json:"target_type,omitempty"`
	Source         string   `json:"source,omitempty"`
	Status         string   `json:"status"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
	DraftRelPath   string   `json:"draft_rel_path,omitempty"`
	Error          string   `json:"error,omitempty"`
}

type DraftWikiQueueAppendResult struct {
	OK              bool                `json:"ok"`
	RepoRoot        string              `json:"repo_root"`
	RepoID          string              `json:"repo_id"`
	ProjectStateDir string              `json:"project_state_dir"`
	Path            string              `json:"path"`
	Event           DraftWikiQueueEvent `json:"event"`
}

type DraftWikiQueueProcessRequest struct {
	RepoRoot        string        `json:"repo_root"`
	AgyCommand      string        `json:"agy_command,omitempty"`
	AgyModel        string        `json:"agy_model,omitempty"`
	AgySettingsPath string        `json:"-"`
	TargetWiki      string        `json:"target_wiki,omitempty"`
	TargetType      string        `json:"target_type,omitempty"`
	Limit           int           `json:"limit,omitempty"`
	Timeout         time.Duration `json:"-"`
}

type DraftWikiQueueProcessResult struct {
	OK              bool                  `json:"ok"`
	Kind            string                `json:"kind"`
	RepoRoot        string                `json:"repo_root"`
	RepoID          string                `json:"repo_id"`
	ProjectStateDir string                `json:"project_state_dir"`
	Path            string                `json:"path"`
	Processed       int                   `json:"processed"`
	Succeeded       int                   `json:"succeeded"`
	Failed          int                   `json:"failed"`
	Warnings        []string              `json:"warnings,omitempty"`
	Events          []DraftWikiQueueEvent `json:"events"`
}

type DraftWikiQueuePruneResult struct {
	OK              bool   `json:"ok"`
	Kind            string `json:"kind"`
	RepoRoot        string `json:"repo_root,omitempty"`
	RepoID          string `json:"repo_id,omitempty"`
	ProjectStateDir string `json:"project_state_dir,omitempty"`
	Path            string `json:"path"`
	Keep            int    `json:"keep"`
	Before          int    `json:"before"`
	After           int    `json:"after"`
	Pruned          int    `json:"pruned"`
}

type DraftWikiQueuePruneAllResult struct {
	OK       bool                        `json:"ok"`
	Kind     string                      `json:"kind"`
	StateDir string                      `json:"state_dir"`
	Keep     int                         `json:"keep"`
	Before   int                         `json:"before"`
	After    int                         `json:"after"`
	Pruned   int                         `json:"pruned"`
	Queues   []DraftWikiQueuePruneResult `json:"queues"`
}
