package guard

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var secretPathRe = regexp.MustCompile(`(?i)(^|/)(\.env(\.|$)|id_rsa|id_dsa|id_ecdsa|id_ed25519|.*\.pem$|.*\.key$|.*\.p12$|.*\.pfx$|.*credentials.*|.*secret.*)`)

func absOrOriginal(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func gitCmd(dir string, args ...string) (int, string, string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String())
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String())
	}
	return 1, strings.TrimSpace(stdout.String()), err.Error()
}

func gitOut(dir string, args ...string) string {
	code, out, _ := gitCmd(dir, args...)
	if code != 0 {
		return ""
	}
	return strings.TrimSpace(out)
}

func splitLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

func stringSet(items ...string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		out[item] = true
	}
	return out
}

func uniqSorted(in []string) []string {
	m := map[string]bool{}
	for _, v := range in {
		if v != "" {
			m[v] = true
		}
	}
	out := make([]string, 0, len(m))
	for v := range m {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
