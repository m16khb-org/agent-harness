package lifecycle

import "fmt"

func SourceCheckoutMisdirectWarning(req HookToolUseLifecycleRequest) string {
	if !toolUseMayMutateLifecycleFiles(req.Tool, req.Command) {
		return ""
	}
	targets := worktreeGuardEditTargets(req)
	records, err := executionV1GuardRecords(req, targets)
	if err != nil {
		return "IssueOps v1 authority state를 검증할 수 없어 source-checkout mutation 진단을 완료하지 못했습니다. `agent-harness doctor --repo " + cleanAbsPath(req.Repo) + " --json`을 실행하세요."
	}
	for _, record := range records {
		if record.Execution == nil {
			continue
		}
		source := cleanAbsPath(record.Execution.Workspace.SourceRoot)
		for _, target := range targets {
			if pathWithin(cleanAbsPath(target), source) {
				return fmt.Sprintf("편집이 IssueOps 실행 %s의 소스 체크아웃 %s에 적용되었습니다. 실행 변경은 canonical worktree %s에서 generation %d의 현재 write lease holder만 수행해야 합니다. `agent-harness issueops execution status --id %s --json`으로 상태를 확인하세요.", record.ID, source, cleanAbsPath(record.Execution.Workspace.Root), record.Execution.Lease.Generation, record.ID)
			}
		}
	}
	return ""
}
