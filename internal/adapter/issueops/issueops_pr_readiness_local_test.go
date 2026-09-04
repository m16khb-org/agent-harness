package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
)

// local readiness는 단계 분류처럼 자주 불리므로 원격을 때리면 안 된다.
func TestIssueOpsLocalPRReadinessNeverFetches(t *testing.T) {
	var commands []string
	restore := stubIssueOpsGit(t, &commands)
	defer restore()

	record := issueops.IssueOpsRecord{
		ID: "io-local", Repo: t.TempDir(), Branch: "12-local",
		Phase: issueops.IssueOpsPhaseAISlopClean,
	}
	ready := IssueOpsLocalPRReadiness(record)

	for _, command := range commands {
		if strings.HasPrefix(command, "fetch") {
			t.Fatalf("local readiness must not fetch, ran %q", command)
		}
	}
	for _, key := range ready.Missing {
		if key == "upstream_fetch" || key == "upstream_synced" {
			t.Fatalf("local readiness must not judge upstream sync, got %v", ready.Missing)
		}
	}
	if ready.Strict {
		t.Fatal("local readiness is not the strict surface")
	}
}

// strict는 같은 관측에 fetch와 동기화 판정을 더한 것이다.
func TestIssueOpsStrictPRReadinessStillFetches(t *testing.T) {
	var commands []string
	restore := stubIssueOpsGit(t, &commands)
	defer restore()

	record := issueops.IssueOpsRecord{
		ID: "io-strict", Repo: t.TempDir(), Branch: "12-strict",
		Phase: issueops.IssueOpsPhaseAISlopClean,
	}
	ready := IssueOpsStrictPRReadiness(record)
	if !ready.Strict {
		t.Fatal("strict readiness must report itself as strict")
	}
	fetched := false
	for _, command := range commands {
		if strings.HasPrefix(command, "fetch") {
			fetched = true
		}
	}
	if !fetched {
		t.Fatalf("strict readiness must fetch, ran %v", commands)
	}
}

// stubIssueOpsGit은 git 관측을 기록만 하는 스텁으로 바꾼다. upstream이 있는
// 것처럼 보여야 fetch 경로에 들어간다.
func stubIssueOpsGit(t *testing.T, commands *[]string) func() {
	t.Helper()
	previousCmd, previousOut := GitCmd, GitOut
	GitCmd = func(dir string, args ...string) (int, string, string) {
		*commands = append(*commands, strings.Join(args, " "))
		switch {
		case len(args) > 0 && args[0] == "rev-parse" && len(args) > 1 && args[1] == "--is-inside-work-tree":
			return 0, "true", ""
		default:
			return 0, "", ""
		}
	}
	GitOut = func(dir string, args ...string) string {
		*commands = append(*commands, strings.Join(args, " "))
		joined := strings.Join(args, " ")
		switch {
		case strings.HasPrefix(joined, "rev-parse --abbrev-ref --symbolic-full-name"):
			return "origin/12-local"
		case joined == "rev-list --left-right --count HEAD...@{u}":
			return "0\t0"
		default:
			return ""
		}
	}
	return func() { GitCmd, GitOut = previousCmd, previousOut }
}
