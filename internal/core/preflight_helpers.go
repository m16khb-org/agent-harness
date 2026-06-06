package core

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var conventionalSubjectRe = regexp.MustCompile(`^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([^)]+\))?!?: .+`)

func listRemotes(root string) []RemoteInfo {
	lines := splitLines(GitOut(root, "remote", "-v"))
	var out []RemoteInfo
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[2] == "(fetch)" {
			out = append(out, RemoteInfo{Name: fields[0], URL: redactRemote(fields[1])})
		}
	}
	return out
}

func recentCommits(root string, limit int) []CommitInfo {
	lines := splitLines(GitOut(root, "log", fmt.Sprintf("-%d", limit), "--pretty=format:%h%x09%s"))
	var out []CommitInfo
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			out = append(out, CommitInfo{SHA: parts[0], Subject: parts[1]})
		}
	}
	return out
}

func commitStyleHints(root, harnessRoot string, limit int) map[string]any {
	recent := recentCommits(root, limit)
	conv := 0
	for _, c := range recent {
		if conventionalSubjectRe.MatchString(c.Subject) {
			conv++
		}
	}
	bodies := strings.Split(GitOut(root, "log", fmt.Sprintf("-%d", limit), "--pretty=format:%B%x1e"), "\x1e")
	lore := 0
	for _, body := range bodies {
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if line == "Lore:" || strings.HasPrefix(line, "Lore-") {
				lore++
				break
			}
		}
	}
	return map[string]any{
		"recent_count":            len(recent),
		"conventional_subjects":   conv,
		"lore_bodies":             lore,
		"recommended":             "conventional_subject_plus_lore_body",
		"message_policy_doc_path": filepath.Join(harnessRoot, ".agent-harness", "COMMIT_POLICY.md"),
	}
}

func redactRemote(url string) string {
	httpUserInfo := regexp.MustCompile(`(https?://)[^/@]+@`)
	url = httpUserInfo.ReplaceAllString(url, `${1}<redacted>@`)
	creds := regexp.MustCompile(`(://)([^:/@]+):([^/@]+)@`)
	return creds.ReplaceAllString(url, `${1}<redacted>:<redacted>@`)
}

func splitLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

func atoi(s string) int {
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}
