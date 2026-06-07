package queue

import (
	"time"
)

const File = "draft-wiki-queue.jsonl"

const LockFile = "draft-wiki-queue.lock"

const MaxEvents = 200

type AppendRequest struct {
	RepoRoot       string   `json:"repo_root"`
	Tool           string   `json:"tool,omitempty"`
	Command        string   `json:"command,omitempty"`
	Paths          []string `json:"paths,omitempty"`
	SourceMaterial string   `json:"source_material"`
	TargetWiki     string   `json:"target_wiki,omitempty"`
	TargetType     string   `json:"target_type,omitempty"`
	Source         string   `json:"source,omitempty"`
}

type Event struct {
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

type AppendResult struct {
	OK              bool   `json:"ok"`
	RepoRoot        string `json:"repo_root"`
	RepoID          string `json:"repo_id"`
	ProjectStateDir string `json:"project_state_dir"`
	Path            string `json:"path"`
	Event           Event  `json:"event"`
}

type ProcessRequest struct {
	RepoRoot        string        `json:"repo_root"`
	AgyCommand      string        `json:"agy_command,omitempty"`
	AgyModel        string        `json:"agy_model,omitempty"`
	AgySettingsPath string        `json:"-"`
	TargetWiki      string        `json:"target_wiki,omitempty"`
	TargetType      string        `json:"target_type,omitempty"`
	Limit           int           `json:"limit,omitempty"`
	Timeout         time.Duration `json:"-"`
}

type ProcessResult struct {
	OK              bool     `json:"ok"`
	Kind            string   `json:"kind"`
	RepoRoot        string   `json:"repo_root"`
	RepoID          string   `json:"repo_id"`
	ProjectStateDir string   `json:"project_state_dir"`
	Path            string   `json:"path"`
	Processed       int      `json:"processed"`
	Succeeded       int      `json:"succeeded"`
	Failed          int      `json:"failed"`
	Warnings        []string `json:"warnings,omitempty"`
	Events          []Event  `json:"events"`
}

type PruneResult struct {
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

type PruneAllResult struct {
	OK       bool          `json:"ok"`
	Kind     string        `json:"kind"`
	StateDir string        `json:"state_dir"`
	Keep     int           `json:"keep"`
	Before   int           `json:"before"`
	After    int           `json:"after"`
	Pruned   int           `json:"pruned"`
	Queues   []PruneResult `json:"queues"`
}
