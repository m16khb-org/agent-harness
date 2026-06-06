package projectdocs

type ProjectDocsRouteResult struct {
	OK          bool                   `json:"ok"`
	Kind        string                 `json:"kind"`
	RepoRoot    string                 `json:"repo_root"`
	Task        string                 `json:"task"`
	GeneratedAt string                 `json:"generated_at"`
	Docs        []ProjectDocRouteEntry `json:"docs"`
	Warnings    []string               `json:"warnings,omitempty"`
}

type ProjectDocRouteEntry struct {
	RelPath string `json:"rel_path"`
	Path    string `json:"path"`
	Reason  string `json:"reason"`
	Exists  bool   `json:"exists"`
}

type routeDoc struct{ rel, reason string }
