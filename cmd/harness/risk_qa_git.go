package main

import (
	"os/exec"
	"path/filepath"
	"strings"
)

func gitChangedPaths(root string) ([]string, []string) {
	cmd := exec.Command("git", "-C", root, "status", "--short", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, []string{"git status unavailable: " + err.Error()}
	}
	paths := []string{}
	for _, line := range strings.Split(string(out), "\n") {
		path := parseGitStatusPath(line)
		if path != "" {
			paths = append(paths, path)
		}
	}
	return uniqueSortedStrings(paths), nil
}

func parseGitStatusPath(line string) string {
	line = strings.TrimRight(line, "\r")
	if strings.TrimSpace(line) == "" {
		return ""
	}
	if len(line) > 3 {
		line = line[3:]
	} else {
		line = strings.TrimSpace(line)
	}
	if strings.Contains(line, " -> ") {
		parts := strings.Split(line, " -> ")
		line = parts[len(parts)-1]
	}
	line = strings.Trim(line, ` "`)
	return filepath.ToSlash(line)
}
