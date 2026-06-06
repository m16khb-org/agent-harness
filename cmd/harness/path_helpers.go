package main

import (
	"path/filepath"

	"agent-harness/cmd/harness/pathutil"
	"agent-harness/internal/core"
)

func readHarnessFile(parts ...string) (string, error) {
	return pathutil.ReadHarnessFile(harnessRoot(), parts...)
}

func harnessRoot() string {
	return pathutil.HarnessRoot(filepath.Join("skills", skillName, "SKILL.md"))
}

func findUp(start, marker string) (string, bool) {
	return pathutil.FindUp(start, marker)
}

func resolveTarget(arg string) string {
	return pathutil.ResolveTarget(arg)
}

func exists(path string) bool {
	return pathutil.Exists(path)
}

func splitLines(s string) []string {
	return pathutil.SplitLines(s)
}

func splitCSV(s string) []string {
	return pathutil.SplitCSV(s)
}

func containsString(items []string, want string) bool {
	return pathutil.ContainsString(items, want)
}

func stateDoctorHasIssueCode(issues []core.StateDoctorIssue, want string) bool {
	return pathutil.StateDoctorHasIssueCode(issues, want)
}
