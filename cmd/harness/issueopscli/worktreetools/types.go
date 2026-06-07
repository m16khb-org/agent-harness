package worktreetools

type PrepareResult struct {
	OK                   bool     `json:"ok"`
	ID                   string   `json:"id"`
	WorktreePath         string   `json:"worktree_path"`
	PackageManager       string   `json:"package_manager,omitempty"`
	DependenciesChecked  bool     `json:"dependencies_checked,omitempty"`
	DependenciesReady    bool     `json:"dependencies_ready,omitempty"`
	DependenciesAction   string   `json:"dependencies_action,omitempty"`
	CodeGraphProjectPath string   `json:"codegraph_project_path"`
	CodeGraphChecked     bool     `json:"codegraph_checked"`
	CodeGraphInitialized bool     `json:"codegraph_initialized,omitempty"`
	CodeGraphReady       bool     `json:"codegraph_ready"`
	Messages             []string `json:"messages,omitempty"`
}
