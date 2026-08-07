package policy

import (
	"os"
	"path/filepath"
	"strings"
)

func absOrOriginal(path string) string {
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func canonicalPotentialPath(path string) string {
	if path == "" {
		return ""
	}
	abs := absOrOriginal(path)
	if eval, err := filepath.EvalSymlinks(abs); err == nil {
		return eval
	}
	originalAbs := abs
	missing := []string{}
	for {
		parent := filepath.Dir(abs)
		if parent == abs {
			return originalAbs
		}
		missing = append([]string{filepath.Base(abs)}, missing...)
		if eval, err := filepath.EvalSymlinks(parent); err == nil {
			parts := append([]string{eval}, missing...)
			return filepath.Join(parts...)
		}
		abs = parent
	}
}

func sameOrWithin(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func commandReferencesOutsideWorkspace(root, cwd string, argv []string) bool {
	if root == "" || cwd == "" || len(argv) < 2 {
		return false
	}
	for _, arg := range argv[1:] {
		for _, candidate := range policyPathCandidates(arg) {
			resolved := resolvePolicyPathCandidate(cwd, candidate)
			if resolved == "" {
				continue
			}
			if !sameOrWithin(root, canonicalPotentialPath(resolved)) {
				return true
			}
		}
	}
	return false
}

func policyPathCandidates(arg string) []string {
	arg = strings.TrimSpace(arg)
	if arg == "" || looksLikeRemoteOrURL(arg) {
		return nil
	}
	if strings.HasPrefix(arg, "-") {
		if key, value, ok := strings.Cut(arg, "="); ok && strings.TrimSpace(key) != "" && policyArgLooksPathLike(value) {
			return []string{value}
		}
		return nil
	}
	if !policyArgLooksPathLike(arg) {
		return nil
	}
	return []string{arg}
}

func policyArgLooksPathLike(arg string) bool {
	arg = strings.TrimSpace(arg)
	if arg == "" || looksLikeRemoteOrURL(arg) {
		return false
	}
	if arg == "~" || strings.HasPrefix(arg, "~/") || strings.HasPrefix(arg, "~"+string(os.PathSeparator)) {
		return true
	}
	if filepath.IsAbs(arg) || arg == "." || arg == ".." {
		return true
	}
	slashArg := filepath.ToSlash(arg)
	return strings.HasPrefix(slashArg, "./") || strings.HasPrefix(slashArg, "../") || strings.Contains(slashArg, "/")
}

func looksLikeRemoteOrURL(arg string) bool {
	lower := strings.ToLower(arg)
	if strings.Contains(lower, "://") {
		return true
	}
	if at := strings.Index(arg, "@"); at >= 0 {
		return strings.Contains(arg[at+1:], ":")
	}
	return false
}

func resolvePolicyPathCandidate(cwd, candidate string) string {
	if strings.TrimSpace(candidate) == "" || strings.HasPrefix(candidate, "~") {
		if candidate == "~" || strings.HasPrefix(candidate, "~/") || strings.HasPrefix(candidate, "~"+string(os.PathSeparator)) {
			home, err := os.UserHomeDir()
			if err != nil || home == "" {
				return candidate
			}
			rest := strings.TrimPrefix(strings.TrimPrefix(candidate, "~/"), "~"+string(os.PathSeparator))
			if rest == candidate {
				rest = ""
			}
			return filepath.Join(home, rest)
		}
		return candidate
	}
	if filepath.IsAbs(candidate) {
		return candidate
	}
	return filepath.Join(cwd, candidate)
}
