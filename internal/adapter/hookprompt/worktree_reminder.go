package hookprompt

import (
	"strconv"
	"strings"
)

func activeWorktreeReminderValue(repo string) string {
	records := ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo)
	worktrees := []string{}
	for _, record := range records {
		if !IssueOpsPhaseExpectsWorktree(record.Phase) {
			continue
		}
		worktree := strings.TrimSpace(record.WorktreePath)
		if worktree == "" {
			continue
		}
		worktrees = append(worktrees, worktree)
	}
	if len(worktrees) == 0 {
		return ""
	}
	value := worktrees[0]
	if len(worktrees) > 1 {
		value += " 외 " + strconv.Itoa(len(worktrees)-1) + "개"
	}
	return value + " - 편집 전 cwd/절대경로 확인"
}
