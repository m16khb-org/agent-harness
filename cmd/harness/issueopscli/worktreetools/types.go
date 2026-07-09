package worktreetools

import "agent-harness/internal/core"

type PrepareResult struct {
	OK                  bool     `json:"ok"`
	ID                  string   `json:"id"`
	WorktreePath        string   `json:"worktree_path"`
	PackageManager      string   `json:"package_manager,omitempty"`
	DependenciesChecked bool     `json:"dependencies_checked,omitempty"`
	DependenciesReady   bool     `json:"dependencies_ready,omitempty"`
	DependenciesAction  string   `json:"dependencies_action,omitempty"`
	Messages            []string `json:"messages,omitempty"`
	PreparedAt          string   `json:"prepared_at,omitempty"`
	Guidance            string   `json:"guidance,omitempty"`
}

func (r PrepareResult) IssueOpsWorktreeToolPreparation() core.IssueOpsWorktreeToolPreparation {
	return core.IssueOpsWorktreeToolPreparation{
		OK:                  r.OK,
		ID:                  r.ID,
		WorktreePath:        r.WorktreePath,
		PackageManager:      r.PackageManager,
		DependenciesChecked: r.DependenciesChecked,
		DependenciesReady:   r.DependenciesReady,
		DependenciesAction:  r.DependenciesAction,
		Messages:            append([]string{}, r.Messages...),
		PreparedAt:          r.PreparedAt,
	}
}
