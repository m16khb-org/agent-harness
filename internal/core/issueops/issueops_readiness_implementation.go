package issueops

import (
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/core/issueops/pathutil"
	"agent-harness/internal/core/preflight"
)

func issueOpsHasImplementationEvidence(record IssueOpsRecord) bool {
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" || !issueOpsWorktreePathValid(worktree) {
		return false
	}
	if code, out, _ := preflight.GitCmd(worktree, "rev-parse", "--is-inside-work-tree"); code == 0 && strings.TrimSpace(out) == "true" {
		if issueOpsGitStatusHasImplementationChange(record, worktree) {
			return true
		}
		return issueOpsGitHeadDiffersFromBase(record, worktree)
	}
	return issueOpsFileTreeHasImplementationChange(record, worktree)
}

func issueOpsGitStatusHasImplementationChange(record IssueOpsRecord, worktree string) bool {
	out := preflight.GitOut(worktree, "status", "--porcelain=v1", "--untracked-files=all")
	for _, line := range strings.Split(out, "\n") {
		path := issueOpsPorcelainPath(line)
		if path == "" {
			continue
		}
		if !issueOpsPathMatchesPlan(record, worktree, path) {
			return true
		}
	}
	return false
}

func issueOpsGitHeadDiffersFromBase(record IssueOpsRecord, worktree string) bool {
	base := ""
	if record.BranchPrepare != nil {
		base = strings.TrimSpace(record.BranchPrepare.BaseBranch)
	}
	if base == "" {
		return false
	}
	for _, ref := range []string{"origin/" + base, base} {
		if code, _, _ := preflight.GitCmd(worktree, "rev-parse", "--verify", ref+"^{commit}"); code != 0 {
			continue
		}
		_, names, _ := preflight.GitCmd(worktree, "diff", "--name-only", ref+"..HEAD", "--")
		for _, name := range strings.Split(names, "\n") {
			name = strings.TrimSpace(name)
			if name != "" && !issueOpsPathMatchesPlan(record, worktree, name) {
				return true
			}
		}
		return false
	}
	return false
}

func issueOpsFileTreeHasImplementationChange(record IssueOpsRecord, worktree string) bool {
	found := false
	_ = filepath.WalkDir(worktree, func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !issueOpsPathMatchesPlan(record, worktree, path) {
			found = true
		}
		return nil
	})
	return found
}

func issueOpsPorcelainPath(line string) string {
	line = strings.TrimRight(line, "\r")
	if len(line) < 4 {
		return ""
	}
	path := strings.TrimSpace(line[3:])
	if renamed := strings.LastIndex(path, " -> "); renamed >= 0 {
		path = strings.TrimSpace(path[renamed+4:])
	}
	return strings.Trim(path, `"`)
}

func issueOpsPathMatchesPlan(record IssueOpsRecord, worktree, path string) bool {
	planPath := strings.TrimSpace(record.PlanPath)
	if planPath == "" || path == "" {
		return false
	}
	if !filepath.IsAbs(planPath) {
		planPath = filepath.Join(worktree, filepath.FromSlash(planPath))
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(worktree, filepath.FromSlash(path))
	}
	planPath = pathutil.CleanAbsPath(planPath)
	path = pathutil.CleanAbsPath(path)
	return path == planPath
}
