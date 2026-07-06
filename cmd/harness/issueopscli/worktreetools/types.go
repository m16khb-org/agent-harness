package worktreetools

import "agent-harness/internal/core"

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
	PreparedAt           string   `json:"prepared_at,omitempty"`
	Guidance             string   `json:"guidance,omitempty"`
}

func (r PrepareResult) IssueOpsWorktreeToolPreparation() core.IssueOpsWorktreeToolPreparation {
	return core.IssueOpsWorktreeToolPreparation{
		OK:                   r.OK,
		ID:                   r.ID,
		WorktreePath:         r.WorktreePath,
		PackageManager:       r.PackageManager,
		DependenciesChecked:  r.DependenciesChecked,
		DependenciesReady:    r.DependenciesReady,
		DependenciesAction:   r.DependenciesAction,
		CodeGraphProjectPath: r.CodeGraphProjectPath,
		CodeGraphChecked:     r.CodeGraphChecked,
		CodeGraphInitialized: r.CodeGraphInitialized,
		CodeGraphReady:       r.CodeGraphReady,
		Messages:             append([]string{}, r.Messages...),
		PreparedAt:           r.PreparedAt,
	}
}
