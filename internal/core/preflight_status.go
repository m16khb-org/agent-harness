package core

import "strings"

func parseGitStatus(lines []string) (staged, unstaged, untracked, secretLike []string) {
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "## ") {
			continue
		}
		if len(line) < 3 {
			continue
		}
		status := line[:2]
		path := strings.TrimSpace(line[3:])
		if strings.Contains(path, " -> ") {
			parts := strings.SplitN(path, " -> ", 2)
			path = parts[1]
		}
		if status == "??" {
			untracked = append(untracked, path)
		} else {
			if status[0] != ' ' {
				staged = append(staged, path)
			}
			if status[1] != ' ' {
				unstaged = append(unstaged, path)
			}
		}
		if secretPathRe.MatchString(path) {
			secretLike = append(secretLike, path)
		}
	}
	return uniqSorted(staged), uniqSorted(unstaged), uniqSorted(untracked), uniqSorted(secretLike)
}
