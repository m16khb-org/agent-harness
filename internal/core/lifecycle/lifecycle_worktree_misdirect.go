package lifecycle

import (
	"strconv"
	"strings"
)

func SourceCheckoutMisdirectWarning(req HookToolUseLifecycleRequest) string {
	if !toolUseMayMutateLifecycleFiles(req.Tool, req.Command) {
		return ""
	}
	repo := cleanAbsPath(req.Repo)
	if repo == "" {
		return ""
	}
	targets := worktreeGuardEditTargets(req)
	if len(targets) == 0 {
		return ""
	}
	sourceTarget := false
	for _, target := range targets {
		cleanTarget := cleanAbsPath(target)
		if cleanTarget == "" {
			continue
		}
		if pathWithin(cleanTarget, repo) && !isInsideWorktreesPath(cleanTarget) {
			sourceTarget = true
			break
		}
	}
	if !sourceTarget {
		return ""
	}
	records := []IssueOpsRecord{}
	for _, record := range ActiveIssueOpsLinkedWorktreeCyclesForRepo(repo) {
		if !IssueOpsPhaseExpectsWorktree(record.Phase) {
			continue
		}
		worktree := cleanAbsPath(record.WorktreePath)
		if worktree == "" {
			continue
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		return ""
	}
	if len(records) == 1 {
		record := records[0]
		worktree := cleanAbsPath(record.WorktreePath)
		return "편집이 소스 체크아웃 " + repo + "에 적용되었습니다. 활성 IssueOps 사이클 " + record.ID +
			"가 워크트리 " + worktree + "를 보유 중입니다. 의도한 대상인지 확인하세요. " +
			"이 편집이 사이클 작업이면 워크트리에서 다시 적용하고 소스 체크아웃 변경을 되돌리세요; " +
			"무관한 작업이면 무시하세요. 사이클이 stale이면 `issueops force-release --id " + record.ID + " --reason <why>`로 해제하세요."
	}
	var b strings.Builder
	b.WriteString("편집이 소스 체크아웃 ")
	b.WriteString(repo)
	b.WriteString("에 적용되었습니다. 활성 IssueOps 사이클 ")
	b.WriteString(strconv.Itoa(len(records)))
	b.WriteString("개가 워크트리를 보유 중입니다 [")
	for i, record := range records {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(record.ID)
		b.WriteString(" -> ")
		b.WriteString(cleanAbsPath(record.WorktreePath))
	}
	b.WriteString("]. 의도한 대상인지 확인하세요. 사이클 작업이면 해당 워크트리에서 다시 적용하고 소스 체크아웃 변경을 되돌리세요; 무관한 작업이면 무시하세요. abandoned cycle은 `issueops force-release --id <id> --reason <why>`로 해제하세요.")
	return b.String()
}
