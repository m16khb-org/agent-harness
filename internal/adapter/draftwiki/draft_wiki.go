package draftwiki

const DraftWikiDir = ProjectDocsDir + "/draft-wiki"

var draftWikiStatusDirs = []string{"draft", "approved", "rejected", "exported"}

type DraftWikiInitRequest struct {
	RepoRoot string `json:"repo_root"`
	Write    bool   `json:"write"`
}

type DraftWikiInitResult struct {
	OK          bool                     `json:"ok"`
	Kind        string                   `json:"kind"`
	RepoRoot    string                   `json:"repo_root"`
	DraftDir    string                   `json:"draft_dir"`
	Write       bool                     `json:"write"`
	DryRun      bool                     `json:"dry_run"`
	GeneratedAt string                   `json:"generated_at"`
	Files       []ProjectDocsPlannedFile `json:"files"`
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
