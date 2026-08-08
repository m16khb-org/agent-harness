// Package draftwiki는 draftwiki capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package draftwiki

import projectdoc "agent-harness/internal/domain/projectdoc"

type DraftWikiInitRequest struct {
	RepoRoot string `json:"repo_root"`
	Write    bool   `json:"write"`
}

type DraftWikiInitResult struct {
	OK          bool                                `json:"ok"`
	Kind        string                              `json:"kind"`
	RepoRoot    string                              `json:"repo_root"`
	DraftDir    string                              `json:"draft_dir"`
	Write       bool                                `json:"write"`
	DryRun      bool                                `json:"dry_run"`
	GeneratedAt string                              `json:"generated_at"`
	Files       []projectdoc.ProjectDocsPlannedFile `json:"files"`
}

type DraftWikiListRequest struct {
	RepoRoot string `json:"repo_root"`
}

type DraftWikiListResult struct {
	OK       bool             `json:"ok"`
	Kind     string           `json:"kind"`
	RepoRoot string           `json:"repo_root"`
	DraftDir string           `json:"draft_dir"`
	Drafts   []DraftWikiDraft `json:"drafts"`
}

type DraftWikiDraft struct {
	RelPath    string `json:"rel_path"`
	Path       string `json:"path"`
	Status     string `json:"status"`
	Title      string `json:"title"`
	Source     string `json:"source,omitempty"`
	TargetWiki string `json:"target_wiki,omitempty"`
	TargetType string `json:"target_type,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Bytes      int64  `json:"bytes"`
}

type DraftWikiMoveRequest struct {
	RepoRoot string `json:"repo_root"`
	Path     string `json:"path"`
}

type DraftWikiMoveResult struct {
	OK       bool           `json:"ok"`
	Kind     string         `json:"kind"`
	RepoRoot string         `json:"repo_root"`
	DraftDir string         `json:"draft_dir"`
	From     DraftWikiDraft `json:"from"`
	To       DraftWikiDraft `json:"to"`
}

type DraftWikiPromoteRequest struct {
	RepoRoot string `json:"repo_root"`
	Path     string `json:"path"`
	Confirm  bool   `json:"confirm"`
}

type DraftWikiPromoteResult struct {
	OK            bool            `json:"ok"`
	Kind          string          `json:"kind"`
	RepoRoot      string          `json:"repo_root"`
	DraftDir      string          `json:"draft_dir"`
	DryRun        bool            `json:"dry_run"`
	Confirm       bool            `json:"confirm"`
	Executed      bool            `json:"executed"`
	From          DraftWikiDraft  `json:"from"`
	To            *DraftWikiDraft `json:"to,omitempty"`
	ExportPath    string          `json:"export_path"`
	ExportRel     string          `json:"export_rel"`
	ExportLogPath string          `json:"export_log_path"`
}

type DraftWikiSubmitRequest struct {
	RepoRoot   string `json:"repo_root"`
	DraftPath  string `json:"draft_path"`
	Title      string `json:"title,omitempty"`
	TargetWiki string `json:"target_wiki,omitempty"`
	TargetType string `json:"target_type,omitempty"`
}

type DraftWikiSubmitResult struct {
	OK        bool           `json:"ok"`
	Kind      string         `json:"kind"`
	RepoRoot  string         `json:"repo_root"`
	InputPath string         `json:"input_path"`
	Draft     DraftWikiDraft `json:"draft"`
}

type DraftWikiSuggestRequest struct {
	RepoRoot   string `json:"repo_root"`
	InputPath  string `json:"input_path"`
	Title      string `json:"title"`
	TargetWiki string `json:"target_wiki"`
	TargetType string `json:"target_type"`
}

type DraftWikiSuggestResult struct {
	OK          bool            `json:"ok"`
	Kind        string          `json:"kind"`
	RepoRoot    string          `json:"repo_root"`
	DraftDir    string          `json:"draft_dir"`
	Executed    bool            `json:"executed"`
	InputPath   string          `json:"input_path"`
	Command     string          `json:"command"`
	PromptBytes int             `json:"prompt_bytes"`
	Prompt      string          `json:"prompt,omitempty"`
	Draft       *DraftWikiDraft `json:"draft,omitempty"`
}
