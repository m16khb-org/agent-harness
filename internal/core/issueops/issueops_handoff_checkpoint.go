package issueops

import (
	"fmt"
	"strings"

	"agent-harness/internal/core/issueops/pathutil"
	"agent-harness/internal/core/preflight"
)

func validateHandoffCleanExactCheckpoint(record IssueOpsRecord) error {
	h := currentIssueOpsHandoff(record)
	if h == nil {
		return fmt.Errorf("execution handoff checkpoint is required")
	}
	workerRoot := pathutil.CleanAbsPath(h.WorkerRoot)
	if workerRoot == "" || workerRoot != pathutil.CleanAbsPath(record.WorktreePath) {
		return fmt.Errorf("handoff worker root does not match the linked worktree checkpoint")
	}
	code, branch, _ := preflight.GitCmd(workerRoot, "branch", "--show-current")
	if code != 0 || strings.TrimSpace(branch) == "" || strings.TrimSpace(branch) != strings.TrimSpace(record.Branch) {
		return fmt.Errorf("handoff checkpoint requires the exact worker branch")
	}
	code, head, _ := preflight.GitCmd(workerRoot, "rev-parse", "--verify", "HEAD^{commit}")
	if code != 0 || strings.TrimSpace(head) == "" || strings.TrimSpace(head) != strings.TrimSpace(h.AttemptBaseHead) {
		return fmt.Errorf("handoff checkpoint requires the exact attempt base head")
	}
	code, status, _ := preflight.GitCmd(workerRoot, "status", "--porcelain=v1")
	if code != 0 {
		return fmt.Errorf("handoff checkpoint worktree status is unreadable")
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("handoff checkpoint requires a clean worker worktree")
	}
	return nil
}

func validateHandoffStartCheckpoint(record IssueOpsRecord) error {
	if currentIssueOpsHandoff(record) != nil {
		_, err := validateOwnershipStartCheckpoint(record)
		return err
	}
	return validateHandoffCleanExactCheckpoint(record)
}
