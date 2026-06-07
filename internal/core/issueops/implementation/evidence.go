package implementation

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/pathutil"
	"agent-harness/internal/core/issueops/readinesspaths"
	"agent-harness/internal/core/preflight"
)

func HasEvidence(record model.IssueOpsRecord) bool {
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" || !readinesspaths.WorktreePathValid(worktree) {
		return false
	}
	if code, out, _ := preflight.GitCmd(worktree, "rev-parse", "--is-inside-work-tree"); code == 0 && strings.TrimSpace(out) == "true" {
		if gitStatusHasImplementationChange(record, worktree) {
			return true
		}
		return gitHeadDiffersFromBase(record, worktree)
	}
	return fileTreeHasImplementationChange(record, worktree)
}

func ChangeFingerprint(record model.IssueOpsRecord) string {
	gitRoot := readinesspaths.StrictGitRoot(record)
	if gitRoot == "" {
		return ""
	}
	if code, out, _ := preflight.GitCmd(gitRoot, "rev-parse", "--is-inside-work-tree"); code != 0 || strings.TrimSpace(out) != "true" {
		return ""
	}
	paths := map[string]bool{}
	if base := diffBaseRef(record, gitRoot); base != "" {
		_, names, _ := preflight.GitCmd(gitRoot, "diff", "--name-only", base+"..HEAD", "--")
		for _, name := range strings.Split(names, "\n") {
			if path := cleanRelativePath(name); path != "" {
				paths[path] = true
			}
		}
	}
	status := preflight.GitOut(gitRoot, "status", "--porcelain=v1", "--untracked-files=all")
	for _, line := range strings.Split(status, "\n") {
		if path := cleanRelativePath(PorcelainPath(line)); path != "" {
			paths[path] = true
		}
	}
	if len(paths) == 0 {
		return ""
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	var b strings.Builder
	b.WriteString("issueops-ai-slop-clean:v1\n")
	for _, rel := range ordered {
		abs := filepath.Join(gitRoot, rel)
		info, err := os.Stat(abs)
		if err != nil {
			b.WriteString(rel + "\x00deleted\n")
			continue
		}
		if info.IsDir() {
			continue
		}
		content, err := os.ReadFile(abs)
		if err != nil {
			return ""
		}
		sum := sha256.Sum256(content)
		b.WriteString(rel + "\x00" + hex.EncodeToString(sum[:]) + "\n")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func PorcelainPath(line string) string {
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

func PathMatchesPlan(record model.IssueOpsRecord, worktree, path string) bool {
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

func gitStatusHasImplementationChange(record model.IssueOpsRecord, worktree string) bool {
	out := preflight.GitOut(worktree, "status", "--porcelain=v1", "--untracked-files=all")
	for _, line := range strings.Split(out, "\n") {
		path := PorcelainPath(line)
		if path == "" {
			continue
		}
		if !PathMatchesPlan(record, worktree, path) {
			return true
		}
	}
	return false
}

func gitHeadDiffersFromBase(record model.IssueOpsRecord, worktree string) bool {
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
			if name != "" && !PathMatchesPlan(record, worktree, name) {
				return true
			}
		}
		return false
	}
	return false
}

func fileTreeHasImplementationChange(record model.IssueOpsRecord, worktree string) bool {
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
		if !PathMatchesPlan(record, worktree, path) {
			found = true
		}
		return nil
	})
	return found
}

func diffBaseRef(record model.IssueOpsRecord, gitRoot string) string {
	if record.BranchPrepare == nil {
		return ""
	}
	base := strings.TrimSpace(record.BranchPrepare.BaseBranch)
	if base == "" {
		return ""
	}
	for _, ref := range []string{"origin/" + base, base} {
		if code, _, _ := preflight.GitCmd(gitRoot, "rev-parse", "--verify", ref+"^{commit}"); code == 0 {
			return ref
		}
	}
	return ""
}

func cleanRelativePath(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." || filepath.IsAbs(path) || strings.HasPrefix(path, ".."+string(filepath.Separator)) || path == ".." {
		return ""
	}
	return filepath.ToSlash(path)
}
