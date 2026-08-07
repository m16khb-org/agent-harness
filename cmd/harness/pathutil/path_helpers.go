package pathutil

import (
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/adapter/core"
)

func ReadHarnessFile(root string, parts ...string) (string, error) {
	path := filepath.Join(append([]string{root}, parts...)...)
	b, err := os.ReadFile(path)
	return string(b), err
}

func HarnessRoot(marker string) string {
	envRoot := os.Getenv("HARNESS_ROOT")
	cwd, _ := os.Getwd()
	executable, _ := os.Executable()
	return harnessRootFrom(marker, envRoot, cwd, executable)
}

func harnessRootFrom(marker, envRoot, cwd, executable string) string {
	if envRoot != "" {
		if root, err := filepath.Abs(envRoot); err == nil {
			return root
		}
	}
	var starts []string
	if cwd != "" {
		starts = append(starts, cwd)
	}
	if executable != "" {
		executableDir := filepath.Dir(executable)
		starts = append(starts, executableDir, filepath.Dir(executableDir))
		if resolved, err := filepath.EvalSymlinks(executable); err == nil && resolved != executable {
			resolvedDir := filepath.Dir(resolved)
			starts = append(starts, resolvedDir, filepath.Dir(resolvedDir))
		}
	}
	for _, start := range starts {
		if root, ok := FindUp(start, marker); ok {
			return root
		}
	}
	if cwd != "" {
		return cwd
	}
	return "."
}

func FindUp(start, marker string) (string, bool) {
	d, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		if Exists(filepath.Join(d, marker)) {
			return d, true
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", false
		}
		d = parent
	}
}

func ResolveTarget(arg string) string {
	if arg == "" {
		if env := os.Getenv("CLAUDE_PROJECT_DIR"); env != "" {
			arg = env
		} else if env := os.Getenv("PWD"); env != "" {
			arg = env
		} else if cwd, err := os.Getwd(); err == nil {
			arg = cwd
		} else {
			arg = "."
		}
	}
	abs, err := filepath.Abs(arg)
	if err != nil {
		return arg
	}
	return abs
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func SplitLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

func SplitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func ContainsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func StateDoctorHasIssueCode(issues []core.StateDoctorIssue, want string) bool {
	for _, issue := range issues {
		if issue.Code == want {
			return true
		}
	}
	return false
}
