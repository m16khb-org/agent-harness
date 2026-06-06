package core

import (
	"os"
	"path/filepath"
	"strings"
)

func IssueOpsAISlopCleanReadiness(record IssueOpsRecord) IssueOpsReadiness {
	ready := IssueOpsImplementationReadiness(record)
	missing := append([]string{}, ready.Missing...)
	if !issueOpsHasImplementationEvidence(record) {
		missing = append(missing, "implementation_changes")
	}
	missing = uniqSorted(missing)
	ready.Missing = missing
	ready.Ready = len(missing) == 0
	return ready
}

func IssueOpsImplementationReadiness(record IssueOpsRecord) IssueOpsReadiness {
	missing := issueOpsBaseImplementationMissing(record)
	if path := strings.TrimSpace(record.WorktreePath); path == "" {
		missing = append(missing, "worktree_path")
	} else if !issueOpsWorktreePathValid(path) {
		missing = append(missing, "worktree_exists")
	}
	if strings.TrimSpace(record.PlanPath) != "" && !issueOpsPlanPathExists(issueOpsPlanExistenceRoot(record), record.PlanPath) {
		missing = append(missing, "plan_exists")
	}
	if !issueOpsPlanInLinkedWorktree(record) {
		missing = append(missing, "plan_in_worktree")
	}
	return IssueOpsReadiness{
		OK:           true,
		Ready:        len(missing) == 0,
		Missing:      uniqSorted(missing),
		IssueURL:     record.IssueURL,
		PlanPath:     record.PlanPath,
		WorktreePath: record.WorktreePath,
		Branch:       record.Branch,
	}
}

func issueOpsHasImplementationEvidence(record IssueOpsRecord) bool {
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" || !issueOpsWorktreePathValid(worktree) {
		return false
	}
	if code, out, _ := GitCmd(worktree, "rev-parse", "--is-inside-work-tree"); code == 0 && strings.TrimSpace(out) == "true" {
		if issueOpsGitStatusHasImplementationChange(record, worktree) {
			return true
		}
		return issueOpsGitHeadDiffersFromBase(record, worktree)
	}
	return issueOpsFileTreeHasImplementationChange(record, worktree)
}

func issueOpsCurrentHead(record IssueOpsRecord) string {
	gitRoot := issueOpsStrictGitRoot(record)
	if gitRoot == "" {
		return ""
	}
	if code, out, _ := GitCmd(gitRoot, "rev-parse", "HEAD"); code == 0 {
		return strings.TrimSpace(out)
	}
	return ""
}

func issueOpsGitStatusHasImplementationChange(record IssueOpsRecord, worktree string) bool {
	out := GitOut(worktree, "status", "--porcelain=v1", "--untracked-files=all")
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
		if code, _, _ := GitCmd(worktree, "rev-parse", "--verify", ref+"^{commit}"); code != 0 {
			continue
		}
		_, names, _ := GitCmd(worktree, "diff", "--name-only", ref+"..HEAD", "--")
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
	planPath = cleanAbsPath(planPath)
	path = cleanAbsPath(path)
	return path == planPath
}
