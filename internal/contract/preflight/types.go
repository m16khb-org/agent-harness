// Package preflight는 preflight capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package preflight

type PreflightResult struct {
	OK               bool           `json:"ok"`
	Error            string         `json:"error,omitempty"`
	Path             string         `json:"path,omitempty"`
	Detail           string         `json:"detail,omitempty"`
	RepoRoot         string         `json:"repo_root,omitempty"`
	Branch           string         `json:"branch,omitempty"`
	Head             string         `json:"head,omitempty"`
	Upstream         *string        `json:"upstream"`
	Ahead            *int           `json:"ahead"`
	Behind           *int           `json:"behind"`
	IsClean          bool           `json:"is_clean"`
	StatusLines      []string       `json:"status_lines"`
	Remotes          []RemoteInfo   `json:"remotes"`
	LastCommit       string         `json:"last_commit"`
	RecentCommits    []CommitInfo   `json:"recent_commits"`
	CommitStyleHints map[string]any `json:"commit_style_hints"`
	StagedFiles      []string       `json:"staged_files"`
	UnstagedFiles    []string       `json:"unstaged_files"`
	UntrackedFiles   []string       `json:"untracked_files"`
	SecretLikePaths  []string       `json:"secret_like_paths"`
	Warnings         []string       `json:"warnings"`
}

type RemoteInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type CommitInfo struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
}
