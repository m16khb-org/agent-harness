package core

import (
	"strings"
)

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
