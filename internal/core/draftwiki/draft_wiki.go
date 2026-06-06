package draftwiki

const DraftWikiDir = ProjectDocsDir + "/draft-wiki"

var draftWikiStatusDirs = []string{"draft", "approved", "rejected"}

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
	RepoRoot          string `json:"repo_root"`
	Path              string `json:"path"`
	TargetWiki        string `json:"target_wiki"`
	TargetType        string `json:"target_type"`
	Confirm           bool   `json:"confirm"`
	LLMWikiConfigPath string `json:"-"`
}

type DraftWikiPromoteResult struct {
	OK             bool           `json:"ok"`
	Kind           string         `json:"kind"`
	RepoRoot       string         `json:"repo_root"`
	DraftDir       string         `json:"draft_dir"`
	DryRun         bool           `json:"dry_run"`
	Confirm        bool           `json:"confirm"`
	Executed       bool           `json:"executed"`
	UpstreamTool   string         `json:"upstream_tool"`
	HandoffCommand string         `json:"handoff_command"`
	HandoffArgs    []string       `json:"handoff_args"`
	LLMWikiRoot    string         `json:"llm_wiki_root,omitempty"`
	LLMWikiRawPath string         `json:"llm_wiki_raw_path,omitempty"`
	LLMWikiRawRel  string         `json:"llm_wiki_raw_rel,omitempty"`
	LLMWikiLogPath string         `json:"llm_wiki_log_path,omitempty"`
	From           DraftWikiDraft `json:"from"`
}
