package core

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var conventionalSubjectRe = regexp.MustCompile(`^(build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test)(\([^)]+\))?!?: .+`)

type PreflightResult struct {
	OK               bool           `json:"ok"`
	Error            string         `json:"error,omitempty"`
	Path             string         `json:"path,omitempty"`
	Detail           string         `json:"detail,omitempty"`
	RepoRoot         string         `json:"repo_root,omitempty"`
	Branch           string         `json:"branch,omitempty"`
	Head             string         `json:"head,omitempty"`
	Upstream         *string        `json:"upstream"`
	Ahead            *int           `json:"ahead"`
	Behind           *int           `json:"behind"`
	IsClean          bool           `json:"is_clean"`
	StatusLines      []string       `json:"status_lines"`
	Remotes          []RemoteInfo   `json:"remotes"`
	LastCommit       string         `json:"last_commit"`
	RecentCommits    []CommitInfo   `json:"recent_commits"`
	CommitStyleHints map[string]any `json:"commit_style_hints"`
	StagedFiles      []string       `json:"staged_files"`
	UnstagedFiles    []string       `json:"unstaged_files"`
	UntrackedFiles   []string       `json:"untracked_files"`
	SecretLikePaths  []string       `json:"secret_like_paths"`
	Warnings         []string       `json:"warnings"`
}

type RemoteInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type CommitInfo struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
}

func GitPreflight(target, harnessRoot string) PreflightResult {
	code, root, stderr := GitCmd(target, "rev-parse", "--show-toplevel")
	if code != 0 {
		return PreflightResult{OK: false, Error: "not_git_repo", Path: target, Detail: stderr, Upstream: nil, Ahead: nil, Behind: nil}
	}
	root = strings.TrimSpace(root)
	branch := GitOut(root, "branch", "--show-current")
	head := GitOut(root, "rev-parse", "--short", "HEAD")
	up := GitOut(root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	var upstream *string
	if up != "" {
		upstream = &up
	}
	status := splitLines(GitOut(root, "status", "--porcelain=v1", "--branch"))
	staged, unstaged, untracked, secretLike := parseGitStatus(status)
	var ahead, behind *int
	if up != "" {
		counts := strings.Fields(GitOut(root, "rev-list", "--left-right", "--count", up+"...HEAD"))
		if len(counts) >= 2 {
			b := atoi(counts[0])
			a := atoi(counts[1])
			behind = &b
			ahead = &a
		}
	}
	warnings := []string{}
	if branch == "" {
		warnings = append(warnings, "detached_head")
	}
	if upstream == nil {
		warnings = append(warnings, "no_upstream")
	}
	if len(secretLike) > 0 {
		warnings = append(warnings, "secret_like_paths_present")
	}
	return PreflightResult{
		OK:               true,
		RepoRoot:         root,
		Branch:           branch,
		Head:             head,
		Upstream:         upstream,
		Ahead:            ahead,
		Behind:           behind,
		IsClean:          len(staged) == 0 && len(unstaged) == 0 && len(untracked) == 0,
		StatusLines:      status,
		Remotes:          listRemotes(root),
		LastCommit:       GitOut(root, "log", "-1", "--pretty=format:%h %s"),
		RecentCommits:    recentCommits(root, 5),
		CommitStyleHints: commitStyleHints(root, harnessRoot, 10),
		StagedFiles:      staged,
		UnstagedFiles:    unstaged,
		UntrackedFiles:   untracked,
		SecretLikePaths:  secretLike,
		Warnings:         warnings,
	}
}

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
