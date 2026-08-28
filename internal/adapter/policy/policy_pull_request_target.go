package policy

import (
	"strings"

	"agent-harness/internal/domain/commandparse"
)

// PreparedBaseBranchLookup은 워크스페이스에서 진행 중인 IssueOps 사이클이
// 준비해 둔 부모 작업 브랜치를 돌려준다. policy는 IssueOps 어댑터를 직접
// 알지 않으므로 실제 구현은 composition root가 설치한다.
//
// 2026-08-27에 PR target 가드가 사라진 원인이 정확히 이런 배선 파일 하나의
// 소실이었다. 그래서 이 변수는 두 겹으로 지켜진다. composition root에
// 설치되었는지를 harnessapp의 배선 테스트가 확인하고, 설치된 뒤 실제로
// 거부가 나오는지를 EvaluateCommandPolicy 통합 테스트가 확인한다.
var PreparedBaseBranchLookup func(workspace string) (string, bool)

// pullRequestTargetDeny는 PR/MR 생성 명령이 진행 중인 사이클의 부모 작업
// 브랜치를 타겟하지 않을 때 거부 사유와 기대값을 돌려준다.
//
// `issueops branch prepare`가 자식 사이클의 base_branch를 우산 브랜치로
// 고정하므로, 그 사이클에서 열리는 MR의 타겟은 그 값 하나뿐이다. 원격 쓰기는
// 되돌리기 번거로운 바깥 작용이라 사후 검증(target_branch_match)만으로는
// 늦다. 잘못된 타겟의 MR이 이미 열린 뒤에야 걸린다.
//
// 진행 중인 사이클이 없으면 판정하지 않는다. IssueOps 밖에서 여는 일상적인
// PR/MR까지 막으면 가드가 아니라 방해가 된다.
func pullRequestTargetDeny(workspaceRoot, cwd string, argv []string) (reason, expected string) {
	if PreparedBaseBranchLookup == nil {
		return "", ""
	}
	parsed, ok := commandparse.ParseRemotePullRequestCreate(argv)
	if !ok {
		return "", ""
	}
	lookupPath := strings.TrimSpace(cwd)
	if lookupPath == "" {
		lookupPath = strings.TrimSpace(workspaceRoot)
	}
	base, ok := PreparedBaseBranchLookup(lookupPath)
	if !ok {
		return "", ""
	}
	if !parsed.HasBaseFlag || strings.TrimSpace(parsed.BaseBranch) == "" {
		return "pr_target_branch_required", base
	}
	if strings.TrimSpace(parsed.BaseBranch) != base {
		return "pr_target_branch_mismatch", base
	}
	return "", ""
}
