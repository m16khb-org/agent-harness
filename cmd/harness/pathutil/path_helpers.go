package pathutil

import (
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/core"
)

func ReadHarnessFile(root string, parts ...string) (string, error) {
	path := filepath.Join(append([]string{root}, parts...)...)
	b, err := os.ReadFile(path)
	return string(b), err
}

func HarnessRoot(marker string) string {
	if env := os.Getenv("HARNESS_ROOT"); env != "" {
		if root, err := filepath.Abs(env); err == nil {
			return root
		}
	}
	var starts []string
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		d := filepath.Dir(exe)
		starts = append(starts, d, filepath.Dir(d))
	}
	for _, start := range starts {
		if root, ok := FindUp(start, marker); ok {
			return root
		}
	}
	if cwd, err := os.Getwd(); err == nil {
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
