package guard

import (
	guardcontract "agent-harness/internal/contract/guard"
	"os"
	"path/filepath"
	"strings"
)

func guardMode(req guardcontract.GuardCheckRequest) string {
	if req.All {
		return "all"
	}
	if req.Staged {
		return "staged"
	}
	if len(req.Files) > 0 {
		return "files"
	}
	return "staged"
}

func guardTargetFiles(root string, req guardcontract.GuardCheckRequest) []string {
	if len(req.Files) > 0 {
		return cleanGuardFiles(root, req.Files)
	}
	if req.All {
		files := []string{}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			name := d.Name()
			if d.IsDir() {
				if name == ".git" || name == "bin" || name == ".cache" || name == ".codex" || name == ".codegraph" || name == ".omx" || name == ".omc" {
					return filepath.SkipDir
				}
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err == nil && isGuardRelevantPath(rel) {
				files = append(files, filepath.ToSlash(rel))
			}
			return nil
		})
		return uniqSorted(files)
	}
	out := splitLines(gitOut(root, "diff", "--cached", "--name-only", "--diff-filter=ACMR"))
	return cleanGuardFiles(root, out)
}

func cleanGuardFiles(root string, files []string) []string {
	out := []string{}
	for _, file := range files {
		file = filepath.ToSlash(strings.TrimSpace(file))
		if file == "" || strings.HasPrefix(file, "../") || filepath.IsAbs(file) || !isGuardRelevantPath(file) {
			continue
		}
		out = append(out, file)
	}
	return uniqSorted(out)
}

func guardReadFile(root, rel string, staged bool) (string, bool) {
	if staged {
		if code, out, _ := gitCmd(root, "show", ":"+rel); code == 0 {
			return out, true
		}
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", false
	}
	return string(b), true
}
