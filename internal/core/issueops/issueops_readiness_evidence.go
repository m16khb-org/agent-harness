package issueops

import (
	"agent-harness/internal/core/preflight"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func issueOpsChangeFingerprint(record IssueOpsRecord) string {
	gitRoot := issueOpsStrictGitRoot(record)
	if gitRoot == "" {
		return ""
	}
	if code, out, _ := preflight.GitCmd(gitRoot, "rev-parse", "--is-inside-work-tree"); code != 0 || strings.TrimSpace(out) != "true" {
		return ""
	}
	paths := map[string]bool{}
	if base := issueOpsDiffBaseRef(record, gitRoot); base != "" {
		_, names, _ := preflight.GitCmd(gitRoot, "diff", "--name-only", base+"..HEAD", "--")
		for _, name := range strings.Split(names, "\n") {
			if path := cleanIssueOpsRelativePath(name); path != "" {
				paths[path] = true
			}
		}
	}
	status := preflight.GitOut(gitRoot, "status", "--porcelain=v1", "--untracked-files=all")
	for _, line := range strings.Split(status, "\n") {
		if path := cleanIssueOpsRelativePath(issueOpsPorcelainPath(line)); path != "" {
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

func issueOpsDiffBaseRef(record IssueOpsRecord, gitRoot string) string {
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

func cleanIssueOpsRelativePath(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." || filepath.IsAbs(path) || strings.HasPrefix(path, ".."+string(filepath.Separator)) || path == ".." {
		return ""
	}
	return filepath.ToSlash(path)
}
